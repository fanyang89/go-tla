package tests

import "testing"

func TestReflectionMetadataPreservesInitializationRefusal(t *testing.T) {
	m := fromSource(t, `package main;import "reflect";func info()reflect.StructField{f,ok:=reflect.TypeFor[*struct{X int}]().Elem().FieldByName("X");if !ok{panic("missing")};return f};func main(){_=info()}`)
	if !m.HasErrors() {
		t.Fatal("unproved reflection initialization omitted")
	}
	count := 0
	for _, d := range m.Diagnostics {
		if d.Code == "modeled-data-operation" {
			count++
		}
	}
	if count < 2 {
		t.Fatal("reflection operation evidence missing")
	}
}
