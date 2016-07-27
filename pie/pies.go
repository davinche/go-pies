package pie

// Pie is a struct that represents all the data about a particular pie
type Pie struct {
	ID        uint64   `json:"id"`
	Name      string   `json:"name"`
	ImageURL  string   `json:"image_url"`
	Price     float64  `json:"price_per_slice"`
	Slices    int      `json:"slices,omitempty"`
	Labels    []string `json:"labels"`
	Permalink string   `json:"permalink,omitempty"`
}

// RecommendPie is the struct used for recommending a pie
type RecommendPie struct {
	ID    uint64  `json:"id"`
	Price float64 `json:"price_per_slice"`
}

// Pies is a list of pies
type Pies []*Pie

// RecommendPies only holds the necessary to serialize for the recommend endpoint.
// Implements the sort interface to sort by budget.
type RecommendPies []*RecommendPie

func (r RecommendPies) Len() int {
	return len(r)
}

func (r RecommendPies) Less(i, j int) bool {
	return r[i].Price < r[j].Price
}

func (r RecommendPies) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
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
