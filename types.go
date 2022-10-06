/*
  Source Configuration Service
  © 2022 Southwinds Tech Ltd - www.southwinds.io
  Licensed under the Apache License, Version 2.0 at http://www.apache.org/licenses/LICENSE-2.0
  Contributors to this project, hereby assign copyright in this code to the project,
  to be licensed under the same terms as the rest of the code.
*/

package src

import (
	"encoding/json"
	"time"
)

// I the definition of an item
type I struct {
	Key     string    `json:"key"`
	Type    string    `json:"type"`
	Value   []byte    `json:"value"`
	Updated time.Time `json:"updated"`
}

func (i *I) Typed(item any) (result any, err error) {
	err = json.Unmarshal(i.Value, item)
	result = item
	return
}

type IL []I

// Typed returns a typed slice of the requested type
// factory: function that creates an instance of an empty item of the typed slice
func (items IL) Typed(factory func() any) ([]any, error) {
	var ii []any
	for _, item := range items {
		i, err := convert(item, factory)
		if err != nil {
			return nil, err
		}
		ii = append(ii, i)
	}
	return ii, nil
}

func convert(i I, factory func() any) (any, error) {
	t := factory()
	err := json.Unmarshal(i.Value, t)
	return t, err
}

// L the definition of a configuration link
type L struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// T the definition of an item tag
type T struct {
	ItemKey string `json:"item_key,omitempty"`
	Name    string `json:"name"`
	Value   string `json:"value"`
}

// TT the definition of an item type
type TT struct {
	Key    string `json:"key,omitempty"`
	Schema []byte `json:"schema"`
	Proto  []byte `json:"proto"`
}
