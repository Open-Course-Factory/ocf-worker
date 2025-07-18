package models

// ThemeInfo représente les informations d'un thème Slidev
// @Description Informations sur un thème Slidev
type ThemeInfo struct {
	Name        string `json:"name" example:"@slidev/theme-seriph"`
	Version     string `json:"version" example:"0.22.7"`
	Installed   bool   `json:"installed" example:"true"`
	Description string `json:"description" example:"Seriph theme with elegant typography"`
	Homepage    string `json:"homepage" example:"https://github.com/slidevjs/themes"`
} // @name ThemeInfo

// ThemeInstallResponse représente la réponse d'installation de thème
// @Description Réponse après installation d'un thème
type ThemeInstallResponse struct {
	Theme     string `json:"theme" example:"@slidev/theme-seriph"`
	Success   bool   `json:"success" example:"true"`
	Installed bool   `json:"installed" example:"true"`
	Message   string `json:"message,omitempty" example:"Theme installed successfully"`
	Error     string `json:"error,omitempty"`
} // @name ThemeInstallResponse

// ThemeListResponse représente la liste des thèmes disponibles
// @Description Liste des thèmes Slidev avec leur statut d'installation
type ThemeListResponse struct {
	Themes  []ThemeInfo  `json:"themes"`
	Count   int          `json:"count" example:"12"`
	Summary ThemeSummary `json:"summary"`
} // @name ThemeListResponse

// ThemeSummary contient un résumé des thèmes
// @Description Résumé des statistiques des thèmes
type ThemeSummary struct {
	Total     int `json:"total" example:"12"`
	Installed int `json:"installed" example:"3"`
	Available int `json:"available" example:"9"`
} // @name ThemeSummary

// ThemeDetectionResponse représente le résultat de détection de thèmes
// @Description Résultat de la détection automatique des thèmes requis
type ThemeDetectionResponse struct {
	JobID           string      `json:"job_id" example:"550e8400-e29b-41d4-a716-446655440001"`
	MissingThemes   []string    `json:"missing_themes" example:"@slidev/theme-seriph,@slidev/theme-minimal"`
	InstalledThemes []ThemeInfo `json:"installed_themes"`
	DetectedCount   int         `json:"detected_count" example:"2"`
} // @name ThemeDetectionResponse

// ThemeAutoInstallResponse représente le résultat d'installation automatique
// @Description Résultat de l'installation automatique des thèmes manquants
type ThemeAutoInstallResponse struct {
	JobID       string               `json:"job_id" example:"550e8400-e29b-41d4-a716-446655440001"`
	Results     []ThemeInstallResult `json:"results"`
	TotalThemes int                  `json:"total_themes" example:"3"`
	Successful  int                  `json:"successful" example:"2"`
	Failed      int                  `json:"failed" example:"1"`
	Duration    string               `json:"duration" example:"2m30s"`
} // @name ThemeAutoInstallResponse

// ThemeInstallResult représente le résultat d'installation d'un thème
// @Description Détails du résultat d'installation d'un thème spécifique
type ThemeInstallResult struct {
	Theme     string   `json:"theme" example:"@slidev/theme-seriph"`
	Success   bool     `json:"success" example:"true"`
	Installed bool     `json:"installed" example:"true"`
	Error     string   `json:"error,omitempty"`
	Duration  string   `json:"duration" example:"45s"`
	Logs      []string `json:"logs,omitempty"`
	ExitCode  int      `json:"exit_code,omitempty" example:"0"`
} // @name ThemeInstallResult

// ThemeInstallRequest représente une demande d'installation de thème
// @Description Requête pour installer un thème Slidev
type ThemeInstallRequest struct {
	Theme string `json:"theme" example:"@slidev/theme-seriph" binding:"required"`
} // @name ThemeInstallRequest
