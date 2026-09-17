package kernel

// Endpoints describes client URLs independently of the adapter's config format.
type Endpoints struct{ HTTP, SOCKS string }

func (e Endpoints) Preferred() string {
	if e.HTTP != "" {
		return e.HTTP
	}
	return e.SOCKS
}
