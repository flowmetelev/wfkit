package cookie

// Cookie – единый формат для всех cookies.
type Cookie struct {
	Domain   string `json:"domain"`
	Path     string `json:"path"`
	Secure   bool   `json:"secure"`
	Expires  *int64 `json:"expires,omitempty"`
	Name     string `json:"name"`
	Value    string `json:"value"`
	HttpOnly bool   `json:"http_only"`
	SameSite int    `json:"same_site"`
}
