//go:build !linux && !windows && !darwin

package platform

func List() ([]Socket, error)            { return nil, ErrUnsupported }
func Identify(pid int) (Identity, error) { return Identity{}, ErrUnsupported }
func Terminate(id Identity) error        { return ErrUnsupported }

func OpenTarget(pid int) (Target, error) { return nil, ErrUnsupported }
