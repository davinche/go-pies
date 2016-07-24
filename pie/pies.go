package pie

// Pie is a struct that represents all the data about a particular pie
type Pie struct {
	ID       uint64   `json:"id"`
	Name     string   `json:"name"`
	ImageURL string   `json:"image_url"`
	Price    float64  `json:"price_per_slice"`
	Slices   int      `json:"slices,omitempty"`
	Labels   []string `json:"labels"`
}

// Purchases contains information about a particualar user as well as
// the number of slices that user purchased for a specific pie
type Purchases struct {
	Username string `json:"username"`
	Slices   int    `json:"slices"`
}

// Details contains the Pie information as well as the user purchases
type Details struct {
	*Pie
	RemainingSlices int          `json:"remaining_slices"`
	Purchases       []*Purchases `json:"purchases"`
}
