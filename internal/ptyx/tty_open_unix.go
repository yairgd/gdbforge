package ptyx

import "os"

func openSlave(name string) (*os.File, error) {
	return os.OpenFile(name, os.O_RDWR, 0)
}
