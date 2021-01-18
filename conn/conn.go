package conn

import (
	"fmt"
	"io"
	"sync"
)

var (
	driversMu sync.RWMutex
	drivers   = make(map[string]Driver)
)

func init() {
	Register("serial", DriverFunc(openSerial))
	Register("tcp", DriverFunc(openTCP))
}

// Driver is interface for connection backend
type Driver interface {
	Open(address string) (io.ReadWriteCloser, error)
}

// Register new driver backend
func Register(name string, driver Driver) {
	driversMu.Lock()
	defer driversMu.Unlock()
	if driver == nil {
		panic("serial: Register driver is nil")
	}
	if _, dup := drivers[name]; dup {
		panic("serial: Register called twice for driver " + name)
	}
	drivers[name] = driver
}

// Open connection with specific driver backend and address
func Open(name, address string) (io.ReadWriteCloser, error) {
	driversMu.RLock()
	driver, ok := drivers[name]
	driversMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("serial: unknown driver %q", name)
	}
	return driver.Open(address)
}

// DriverFunc convert function into Driver like http.HandlerFunc
type DriverFunc func(address string) (io.ReadWriteCloser, error)

// Open call itsself as function
func (f DriverFunc) Open(address string) (io.ReadWriteCloser, error) {
	return f(address)
}
