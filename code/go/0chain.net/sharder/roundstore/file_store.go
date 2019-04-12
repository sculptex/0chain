package roundstore

import (
	"os"
	"path/filepath"
	"strconv"
)

type FSRoundStore struct {
	RootDirectory string
	fileName      string
	bytes         int
}

func NewFSRoundStore(rootDir string) *FSRoundStore {
	store := &FSRoundStore{RootDirectory: rootDir}
	store.fileName = "round"
	store.bytes = 0
	return store
}

func (frs *FSRoundStore) getFile() string {
	return frs.RootDirectory + string(os.PathSeparator) + frs.fileName + ".txt"
}

func (frs *FSRoundStore) Write(roundNum int64) error {
	file := frs.getFile()
	dir := filepath.Dir(file)
	os.MkdirAll(dir, 0755)
	data := string(roundNum)
	f, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	frs.bytes, err = f.WriteString(data)
	if err != nil {
		return err
	}
	return nil
}

func (frs *FSRoundStore) Read() (int64, error) {
	file := frs.getFile()
	f, err := os.Open(file)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	data := make([]byte, frs.bytes)
	_, err = f.Read(data)
	if err != nil {
		return 0, err
	}
	var value int64
	value, err = strconv.ParseInt(string(data), 10, 64)
	return value, nil
}