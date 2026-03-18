package plan

import (
	"encoding/json"
	"fmt"
	"strings"
)

type Layer struct {
	Image  string `json:"image,omitempty"`
	Step   string `json:"step,omitempty"`
	Local  bool   `json:"local,omitempty"`
	Spread bool   `json:"spread,omitempty"`

	Filter
}

func NewStepLayer(stepName string, filter ...Filter) Layer {
	input := Layer{
		Step: stepName,
	}

	if len(filter) > 0 {
		input.Include = filter[0].Include
		input.Exclude = filter[0].Exclude
	}

	return input
}

func NewImageLayer(image string, filter ...Filter) Layer {
	input := Layer{
		Image: image,
	}

	if len(filter) > 0 {
		input.Include = filter[0].Include
		input.Exclude = filter[0].Exclude
	}

	return input
}

func NewLocalLayer() Layer {
	return Layer{
		Local:  true,
		Filter: NewIncludeFilter([]string{"."}),
	}
}

func (i Layer) IsEmpty() bool {
	return i.Step == "" && i.Image == "" && !i.Local && !i.Spread
}

func (i Layer) IsSpread() bool {
	return i.Spread
}

func (i *Layer) String() string {
	bytes, _ := json.Marshal(i)
	return string(bytes)
}

func (i *Layer) DisplayName() string {
	include := strings.Join(i.Include, ", ")

	if i.Local {
		return fmt.Sprintf("local %s", include)
	}

	if i.Spread {
		return fmt.Sprintf("spread %s", include)
	}

	if i.Step != "" {
		return fmt.Sprintf("$%s", i.Step)
	}

	if i.Image != "" {
		return i.Image
	}

	return fmt.Sprintf("input %s", include)
}

func (i *Layer) UnmarshalJSON(data []byte) error {
	type Alias Layer
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(i),
	}
	if err := json.Unmarshal(data, &aux); err == nil {
		return nil
	}

	str := string(data)
	str = strings.Trim(str, "\"")
	switch str {
	case ".":
		*i = NewLocalLayer()
		return nil
	case "...":
		*i = Layer{Spread: true}
		return nil
	default:
		if strings.HasPrefix(str, "$") {
			stepName := strings.TrimPrefix(str, "$")
			*i = NewStepLayer(stepName)
			return nil
		}
		return fmt.Errorf("invalid input format: %s", str)
	}
}
