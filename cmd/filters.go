package main

import (
	"fmt"
	"time"

	"github.com/flosch/pongo2/v6"
)

func registerTemplateFilters() {
	_ = pongo2.RegisterFilter("elapsed", elapsedFilter)
}

func elapsedFilter(in *pongo2.Value, _ *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	var t time.Time
	switch v := in.Interface().(type) {
	case time.Time:
		t = v
	case *time.Time:
		if v == nil {
			return pongo2.AsValue("—"), nil
		}
		t = *v
	default:

		return pongo2.AsValue("—"), nil
	}

	if t.IsZero() {
		return pongo2.AsValue("—"), nil
	}

	d := time.Since(t)
	if d < 0 {
		d = -d
	}

	var s string
	switch {
	case d < 45*time.Second:
		s = "just now"
	case d < 90*time.Second:
		s = "1 min ago"
	case d < time.Hour:
		s = fmt.Sprintf("%d min ago", int(d.Minutes()))
	case d < 2*time.Hour:
		s = "1 h ago"
	case d < 24*time.Hour:
		s = fmt.Sprintf("%d h ago", int(d.Hours()))
	case d < 48*time.Hour:
		s = "yesterday"
	case d < 7*24*time.Hour:
		s = fmt.Sprintf("%d d ago", int(d.Hours()/24))
	case d < 30*24*time.Hour:
		s = fmt.Sprintf("%d w ago", int(d.Hours()/(24*7)))
	case d < 365*24*time.Hour:
		s = fmt.Sprintf("%d mo ago", int(d.Hours()/(24*30)))
	default:
		s = fmt.Sprintf("%d y ago", int(d.Hours()/(24*365)))
	}
	return pongo2.AsValue(s), nil
}
