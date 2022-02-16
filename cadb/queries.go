package cadb

import (
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"time"
)

// NonUnique is an error that indicates a given serial number was not
// unique.
var NonUnique = errors.New("Non Unique Serial")

// Generate a serial number for a certificate.  The serial number is
// required to be unique for all certificates generated by a given
// authority.  To help with this, we will use a timestamp for the
// signature.  We scan the database to make sure that timestamp is not
// already taken (to protect against clock changes).
func (conn *Conn) GetSerial() (*big.Int, error) {
	for {
		ser, err := conn.tryGetSerial()
		if err != NonUnique {
			// Any other kind of error, or success
			// returns.
			return ser, err
		}

		// We need to sleep enough for time to have changed a
		// bit.  Unfortunately, this would requiring knowing
		// the granularity of the system clock.  We'll start
		// by making an assumption that sleeping will sleep at
		// least long enough for the queried time to have
		// changed.
		time.Sleep(time.Millisecond)
	}
}

// tryGetSerial generates a serial number, based on the current clock,
// queries the database to ensure that it hasn't been issued before.
// If successful, err will be nil, it will return the serial number.
// Otherwise, it will return NonUnique, for which the caller should
// retry after a period of time.  Any other error indicates a failure
// with one of the called routines.
func (conn *Conn) tryGetSerial() (*big.Int, error) {
	// We just use the time's nanosecond resolution.
	ser := big.NewInt(time.Now().UnixNano())

	// TODO: This should be done inside of a transaction, to
	// prevent any kind of race caused by multiple queries.

	row := conn.db.QueryRow(`SELECT COUNT(*) FROM certs WHERE serial = ?`,
		ser.String())
	var count int
	err := row.Scan(&count)
	if err != nil {
		return nil, err
	}

	if count != 0 {
		return nil, NonUnique
	}

	return ser, nil
}

// AddCert adds a newly generated certificate to the database.
func (conn *Conn) AddCert(id string, name string, serial *big.Int, keyId []byte, expiry time.Time, cert []byte) error {
	tx, err := conn.db.Begin()
	if err != nil {
		return err
	}

	// Query the device database to see if we need to create an
	// entry for it.
	row := tx.QueryRow(`SELECT COUNT(*) FROM devices WHERE id = ?`, id)
	var count int
	err = row.Scan(&count)
	if err != nil {
		_ = tx.Rollback()
		return err
	}

	if count == 0 {
		_, err = tx.Exec(`INSERT INTO devices (id, registered) VALUES (?, ?)`,
			id, 0)
		if err != nil {
			_ = tx.Rollback()
			return err
		}
	}

	// Record the certificate as associated with this device.
	_, err = tx.Exec(`INSERT INTO certs (id, name, serial, keyid, expiry, cert, valid) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, name, serial.Int64(), keyId, expiry, cert, 1)
	if err != nil {
		_ = tx.Rollback()
		return err
	}

	err = tx.Commit()
	return err
}

// SerialValid checks if a valid certificate exists for the specified serial
func (conn *Conn) SerialValid(serial *big.Int) (bool, error) {
	var valid bool

	if err := conn.db.QueryRow("SELECT valid FROM certs WHERE serial = ?",
		serial.String()).Scan(&valid); err != nil {
		if err == sql.ErrNoRows {
			return false, fmt.Errorf("serial %d: unknown certificate", serial)
		}
		return false, fmt.Errorf("serial %d: %s", serial, err)
	}
	return valid, nil
}

// UnregisteredDevices returns a list of devices that have not been
// registered with the cloud.  This may need to be extended to return
// certificate information, if we add support for a cloud service that
// does not support signed certificates.
func (conn *Conn) UnregisteredDevices() ([]string, error) {
	var result []string

	rows, err := conn.db.Query(`SELECT devices.id FROM certs LEFT JOIN devices
		WHERE certs.id = devices.id AND registered = 0`)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var id string
		err = rows.Scan(&id)
		if err != nil {
			return nil, err
		}
		result = append(result, id)
	}

	return result, nil
}

// MarkRegistered indicates that the given device has successfully
// been marked as registered with the Cloud service.
func (conn *Conn) MarkRegistered(device string) error {
	tx, err := conn.db.Begin()
	if err != nil {
		return err
	}

	_, err = tx.Exec(`UPDATE devices
		SET registered = 1
		WHERE id = ?`, device)
	if err != nil {
		_ = tx.Rollback()
		return err
	}

	err = tx.Commit()
	return err
}
