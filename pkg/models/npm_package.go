package models

import "io"

// NpmPackageInstallResult représente le résultat d'installation d'un paquet
// @Description Détails du résultat d'installation d'un paquet spécifique
type NpmPackageInstallResult struct {
	Package   string        `json:"package,omitempty" example:"@slidev/theme-seriph"`
	Success   bool          `json:"success" example:"true"`
	Installed bool          `json:"installed" example:"true"`
	Error     string        `json:"error,omitempty"`
	Duration  int64         `json:"duration" example:"45"`
	Logs      []string      `json:"logs,omitempty"`
	ExitCode  int           `json:"exit_code,omitempty" example:"0"`
	Pipes     *InstallPipes `json:"-"` // Non exporté

} // @name ThemeInstallResult

// installPipes structure pour gérer les pipes de manière centralisée
type InstallPipes struct {
	Stdout io.ReadCloser
	Stderr io.ReadCloser
	Stdin  io.WriteCloser
}
