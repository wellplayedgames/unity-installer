// +build windows

package packageinstaller

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-logr/logr"
	"io"
	"os"
	"strings"
	"syscall"
	"unsafe"

	"github.com/Microsoft/go-winio"
	"github.com/google/uuid"
	"github.com/wellplayedgames/unity-installer/pkg/release"
)

const (
	serviceFlag = "--package-service="
)

var (
	dllShell32       = syscall.NewLazyDLL("shell32.dll")
	procShellExecute = dllShell32.NewProc("ShellExecuteExW")

	dllKernel32     = syscall.NewLazyDLL("kernel32.dll")
	procCreateMutex = dllKernel32.NewProc("CreateMutexW")
)

const (
	cSEE_MASK_NOCLOSEPROCESS = 0x00000040
	cSEE_MASK_NO_CONSOLE     = 0x00008000
)

type shellExecuteInfoW struct {
	cbSize       uint32
	fMask        uint32
	hwnd         uintptr
	lpVerb       *uint16
	lpFile       *uint16
	lpParameters *uint16
	lpDirectory  *uint16
	nShow        int32
	hInstApp     uintptr
	lpIDList     uintptr
	lpClass      *uint16
	hkeyClass    uintptr
	dwHotKey     uint32
	hIcon        uintptr
	hProcess     syscall.Handle
}

func shellExecute(verb, file, params string) error {
	lpVerb, err := syscall.UTF16PtrFromString(verb)
	if err != nil {
		return err
	}

	lpFile, err := syscall.UTF16PtrFromString(file)
	if err != nil {
		return err
	}

	lpParameters, err := syscall.UTF16PtrFromString(params)
	if err != nil {
		return err
	}

	info := &shellExecuteInfoW{
		fMask:        cSEE_MASK_NOCLOSEPROCESS | cSEE_MASK_NO_CONSOLE,
		lpVerb:       lpVerb,
		lpFile:       lpFile,
		lpParameters: lpParameters,
		nShow:        0, // SW_HIDE
	}
	info.cbSize = uint32(unsafe.Sizeof(*info))

	ret, _, err := procShellExecute.Call(uintptr(unsafe.Pointer(info)))
	if ret == 0 {
		return err
	}

	s, e := syscall.WaitForSingleObject(info.hProcess, syscall.INFINITE)
	if s != syscall.WAIT_OBJECT_0 {
		return fmt.Errorf("wait failed: %v", e)
	}

	return nil
}

type installerMessage struct {
	PackagePath string                  `json:"packagePath"`
	Destination string                  `json:"destination"`
	Modules     []release.ModuleRelease `json:"modules"`
	Options     release.InstallOptions  `json:",inline"`
}

type responseMessage struct {
	ErrorString string `json:"error"`
}

func readMessages(r io.Reader, handler func([]byte) error) error {
	readSize := 2048
	buf := make([]byte, 0, 2*readSize)

	for {
		l := len(buf)

		if cap(buf) < l+readSize {
			buf2 := make([]byte, l+2*readSize)
			copy(buf2[:l], buf)
			buf = buf2[:l]
		}

		dest := buf[l : l+readSize]
		n, err := r.Read(dest)
		buf = buf[:l+n]

		for {
			idx := bytes.IndexByte(buf, '\n')
			if idx < 0 {
				break
			}

			line := buf[:idx]
			buf = buf[idx+1:]

			err := handler(line)
			if err != nil {
				return err
			}
		}

		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}
	}

	return nil
}

func writeMessages(w io.Writer, messages <-chan interface{}) error {
	e := json.NewEncoder(w)

	for m := range messages {
		err := e.Encode(m)
		if err != nil {
			return err
		}
	}

	return nil
}

type serviceInstaller struct {
	requestChannel  chan<- installerMessage
	responseChannel <-chan responseMessage
}

func NewServiceInstaller(logger logr.Logger) (PackageInstaller, error) {
	pipeName := fmt.Sprintf(`\\.\pipe\UnityInstaller-%s`, uuid.New().String())

	l, err := winio.ListenPipe(pipeName, nil)
	if err != nil {
		return nil, err
	}

	reqCh := make(chan installerMessage)
	tmpCh := make(chan interface{})
	respCh := make(chan responseMessage)

	go func() {
		err := runService(pipeName)
		if err != nil {
			fmt.Printf("error: %v\n", err)
		}
	}()

	c, err := l.Accept()
	if err != nil {
		return nil, err
	}

	go func() {
		defer close(tmpCh)

		for m := range reqCh {
			tmpCh <- m
		}
	}()

	go func() {
		err := writeMessages(c, tmpCh)
		if err != nil {
			panic(err)
		}
	}()

	go func() {
		defer close(respCh)

		err := readMessages(c, func(b []byte) error {
			m := responseMessage{}
			err := json.Unmarshal(b, &m)
			if err != nil {
				return err
			}

			respCh <- m
			return nil
		})
		if err != nil {
			panic(err)
		}
	}()

	return &serviceInstaller{reqCh, respCh}, nil
}

func MaybeHandleService(logger logr.Logger) {
	arg := os.Args[1]
	if !strings.HasPrefix(arg, serviceFlag) {
		return
	}
	defer os.Exit(0)

	logger.Info("starting installer service")
	pipeName := arg[len(serviceFlag):]
	inst := NewLocalInstaller(logger)

	c, err := winio.DialPipe(pipeName, nil)
	if err != nil {
		logger.Error(err, "failed to connect to service pipe", "pipe", pipeName)
		os.Exit(1)
	}

	reqCh := make(chan installerMessage)
	tmpCh := make(chan interface{})
	respCh := make(chan responseMessage)

	go func() {
		defer close(reqCh)

		err := readMessages(c, func(b []byte) error {
			m := installerMessage{}
			err := json.Unmarshal(b, &m)
			if err != nil {
				return err
			}

			reqCh <- m
			return nil
		})
		if err != nil {
			panic(err)
		}
	}()

	go func() {
		defer close(tmpCh)

		for m := range respCh {
			tmpCh <- m
		}
	}()

	go func() {
		err := writeMessages(c, tmpCh)
		if err != nil {
			panic(err)
		}
	}()

	if err := takeMutex("Global\\UnityInstaller"); err != nil {
		logger.Error(err, "failed to acquire mutex")
		os.Exit(1)
	}

	handleInstaller(inst, reqCh, respCh)
}

func NewDefaultInstaller(logger logr.Logger) (PackageInstaller, error) {
	return NewServiceInstaller(logger)
}

func (i *serviceInstaller) Close() error {
	close(i.requestChannel)
	return nil
}

// InstallPackage installs a single Unity package.
func (i *serviceInstaller) StoreModules(destination string, modules []release.ModuleRelease) error {
	if modules == nil {
		modules = []release.ModuleRelease{}
	}

	req := installerMessage{
		Destination: destination,
		Modules:     modules,
	}

	i.requestChannel <- req
	resp, ok := <-i.responseChannel

	if resp.ErrorString != "" {
		return errors.New(resp.ErrorString)
	} else if !ok {
		return io.ErrUnexpectedEOF
	}

	return nil
}

// InstallPackage installs a single Unity package.
func (i *serviceInstaller) InstallPackage(packagePath string, destination string, options release.InstallOptions) error {
	fmt.Printf("installing %s...\n", packagePath)

	req := installerMessage{
		PackagePath: packagePath,
		Destination: destination,
		Options:     options,
	}

	i.requestChannel <- req
	resp, ok := <-i.responseChannel

	if resp.ErrorString != "" {
		return errors.New(resp.ErrorString)
	} else if !ok {
		return io.ErrUnexpectedEOF
	}

	return nil
}

func handleInstaller(inst PackageInstaller, requestChannel <-chan installerMessage, responseChannel chan<- responseMessage) {
	defer close(responseChannel)

	for req := range requestChannel {
		var err error

		if req.Modules != nil {
			err = inst.StoreModules(req.Destination, req.Modules)
		} else {
			err = inst.InstallPackage(req.PackagePath, req.Destination, req.Options)
		}

		resp := responseMessage{}

		if err != nil {
			resp.ErrorString = err.Error()
		}

		responseChannel <- resp
	}
}

func takeMutex(name string) error {
	namePtr, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return err
	}

	ret, _, err := procCreateMutex.Call(0, 1, uintptr(unsafe.Pointer(namePtr)))
	h := syscall.Handle(ret)

	switch err {
	case syscall.Errno(0):
		return nil

	case syscall.ERROR_ALREADY_EXISTS:
		s, e := syscall.WaitForSingleObject(h, syscall.INFINITE)
		if s != syscall.WAIT_OBJECT_0 {
			return fmt.Errorf("wait failed %v: %v", s, e)
		}

		return nil

	default:
		return err
	}
}

func runService(pipeName string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	cmdLine := fmt.Sprintf("\"%s%s\"", serviceFlag, pipeName)
	err = shellExecute("runas", exe, cmdLine)
	return err
}
