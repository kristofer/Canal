// stdlib/fs/fs.go
package fs

import "runtime"

type File struct {
    cap runtime.CapHandle
}

func Open(path string) (*File, error) {
    // Request fs capability from kernel
    cap, err := runtime.RequestCap("fs:" + path, runtime.CapRead)
    if err != nil {
        return nil, err
    }

    return &File{cap: cap}, nil
}

func (f *File) Read(buf []byte) (int, error) {
    return runtime.CapRecv(f.cap, buf)
}

func (f *File) Close() error {
    return runtime.RevokeCap(f.cap)
}
