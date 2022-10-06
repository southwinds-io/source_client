/*
  Source Configuration Service
  © 2022 Southwinds Tech Ltd - www.southwinds.io
  Licensed under the Apache License, Version 2.0 at http://www.apache.org/licenses/LICENSE-2.0
  Contributors to this project, hereby assign copyright in this code to the project,
  to be licensed under the same terms as the rest of the code.
*/

package src

import (
	"fmt"
	"testing"
	"time"
)

func TestAll(t *testing.T) {
	c := New("http://127.0.0.1:8080", "admin", "adm1n", nil)
	// define a json schema for a configuration
	// note you do not need to create the schema, it is inferred from an empty struct in this case I am using
	// ClientOptions{}
	err := c.SetType("AAA", ClientOptions{
		InsecureSkipVerify: true,
		Timeout:            5 * time.Second,
	})
	if err != nil {
		t.Fatalf(err.Error())
	}
	// set a configuration: note the actual value is any object you want, in this case I am using ClientOptions{}
	err = c.Save("OPT_1", "AAA", ClientOptions{
		InsecureSkipVerify: false,
		Timeout:            60,
	})
	if err != nil {
		t.Fatalf(err.Error())
	}
	// retrieve the raw configuration item
	raw, _ := c.LoadRaw("OPT_1")

	// get the typed version of the item
	opts, err := raw.Typed(new(ClientOptions))
	if err != nil {
		t.Fatalf(err.Error())
	}
	fmt.Println(opts)

	// retrieve the typed version at once
	opts, err = c.Load("OPT_1", new(ClientOptions))
	if err != nil {
		t.Fatalf(err.Error())
	}
	fmt.Println(opts)

	// tag the item with a name and also a value
	err = c.Tag("OPT_1", "status", "dev")
	if err != nil {
		t.Fatalf(err.Error())
	}
	// set another item
	err = c.Save("OPT_2", "AAA", ClientOptions{
		InsecureSkipVerify: true,
		Timeout:            120,
	})
	if err != nil {
		t.Fatalf(err.Error())
	}
	// remove the association
	err = c.Unlink("OPT_1", "OPT_2")
	if err != nil {
		t.Fatalf(err.Error())
	}
	// associate the two items
	err = c.Link("OPT_1", "OPT_2")
	if err != nil {
		t.Fatalf(err.Error())
	}
	// remove the tag
	err = c.Untag("OPT_1", "status")
	if err != nil {
		t.Fatalf(err.Error())
	}
	// get the list of raw configuration item children
	items, err := c.LoadChildren(func() any {
		return new(ClientOptions)
	}, "OPT_1")
	if err != nil {
		t.Fatalf(err.Error())
	}
	fmt.Println(items)
}

// TestItemsByType shows how to create multiple configurations and do a strong typed query
func TestItemsByType(t *testing.T) {
	c := New("http://127.0.0.1:8080", "admin", "adm1n", nil)
	// define a json schema for a configuration
	// note you do not need to create the schema, it is inferred from an empty struct in this case I am using
	// ClientOptions{}
	err := c.SetType("AAA", ClientOptions{
		InsecureSkipVerify: true,
		Timeout:            5 * time.Second,
	})
	if err != nil {
		t.Fatalf(err.Error())
	}
	// note the ? in the key name, it will automatically generate a unique time based sequence
	err = c.Save("ITEM_?", "AAA", ClientOptions{
		InsecureSkipVerify: false,
		Timeout:            10,
	})
	c.Save("ITEM_?", "AAA", ClientOptions{
		InsecureSkipVerify: false,
		Timeout:            15,
	})
	c.Save("ITEM_?", "AAA", ClientOptions{
		InsecureSkipVerify: true,
		Timeout:            20,
	})
	items, err := c.LoadItemsByType(func() any {
		// this creates an empty configuration ready for unmarshalling
		// acts as a factory for new configurations that the unmarshaller will add to the result list
		return new(ClientOptions)
	}, "AAA")
	if err != nil {
		t.Fatalf(err.Error())
	}
	for _, item := range items {
		fmt.Printf("%d\n", item.(*ClientOptions).Timeout)
	}
}
