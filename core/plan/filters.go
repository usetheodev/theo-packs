package plan

type Filter struct {
	Include []string `json:"include,omitempty"`
	Exclude []string `json:"exclude,omitempty"`
}

func NewFilter(include []string, exclude []string) Filter {
	return Filter{
		Include: include,
		Exclude: exclude,
	}
}

func NewIncludeFilter(include []string) Filter {
	return Filter{
		Include: include,
	}
}
