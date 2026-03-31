//go:build !linux && !darwin

package hostservice

import "fmt"

func servicePath(projectDir string) (string, string, string, error) {
	return "", "", "", fmt.Errorf("managed services are not supported on this platform")
}

func install(projectDir, executable string) (*Metadata, error) {
	return nil, fmt.Errorf("managed services are not supported on this platform")
}

func uninstall(meta *Metadata) error {
	return fmt.Errorf("managed services are not supported on this platform")
}

func start(meta *Metadata) error {
	return fmt.Errorf("managed services are not supported on this platform")
}

func stop(meta *Metadata) error {
	return fmt.Errorf("managed services are not supported on this platform")
}

func restart(meta *Metadata) error {
	return fmt.Errorf("managed services are not supported on this platform")
}

func status(projectDir string, meta *Metadata) (*Status, error) {
	return &Status{Metadata: meta, Message: "managed services are not supported on this platform"}, nil
}
