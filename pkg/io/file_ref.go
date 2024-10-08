package io

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/afero"
)

type (

	// FileRef is a lightweight representation of a file, deferring reading its contents until `WriteTo` is called.
	FileRef struct {
		FPath          string
		RootConfigPath string
	}
)

func (r *FileRef) Clone() File {
	return r
}

func (r *FileRef) Path() string {
	return r.FPath
}

func (r *FileRef) WriteTo(w io.Writer) (int64, error) {
	f, err := os.Open(filepath.Join(r.RootConfigPath, r.FPath))
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return io.Copy(w, f)
}

func OutputTo(files []File, dest string) error {
	return OutputToFS(afero.NewOsFs(), files, dest)
}

func OutputToFS(fs afero.Fs, files []File, dest string) error {
	errChan := make(chan error)
	for idx := range files {
		go func(f File) {
			path := filepath.Join(dest, f.Path())
			dir := filepath.Dir(path)
			err := fs.MkdirAll(dir, 0777)
			if err != nil {
				errChan <- fmt.Errorf("could not create directory for %s: %w", path, err)
				return
			}
			file, err := fs.OpenFile(path, os.O_RDWR, 0777)
			if os.IsNotExist(err) {
				file, err = fs.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0777)
			} else if err == nil {
				err = file.Truncate(0)
			}
			if err != nil {
				errChan <- fmt.Errorf("could not open file for writing %s: %w", path, err)
				return
			}
			_, err = f.WriteTo(file)
			file.Close()
			if err != nil {
				errChan <- fmt.Errorf("could not write file %s: %w", path, err)
				return
			}
			errChan <- nil
		}(files[idx])
	}

	var errs error
	for i := 0; i < len(files); i++ {
		errs = errors.Join(errs, <-errChan)
	}
	return errs
}
