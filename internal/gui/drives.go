package gui

type DriveInfo struct {
	Letter    string `json:"letter"`
	Path      string `json:"path"`
	Name      string `json:"name"`
	TotalSize int64  `json:"total_size"`
	FreeSize  int64  `json:"free_size"`
}
