package tests

import "testing"

const rotatedScan = `func scan(s string)bool{if len(s)==0{return false};for i:=range len(s){if s[i]<'A'||s[i]>'Z'{return false}};return true};`

func TestTLCRotatedFiniteData(t *testing.T) {
	for _, tc := range []struct{ name, source, want string }{
		{"locked", `package main;import "sync";` + rotatedScan + `func main(){var m sync.Mutex;m.Lock();_=scan("ABC");m.Unlock()}`, "No error has been found"},
		{"empty", `package main;` + rotatedScan + `func main(){c:=make(chan int,1);_=scan("");c<-1;<-c}`, "No error has been found"},
		{"abstract-predicate", `package main;` + rotatedScan + `func main(){c:=make(chan int);if scan("ABC"){close(c)};<-c}`, "Error: Deadlock reached"},
		{"initializer", `package main;` + rotatedScan + `var valid=scan("ABC");func main(){c:=make(chan int);close(c)}`, "No error has been found"},
		{"argument-blocking", `package main;` + rotatedScan + `func input(c chan int)string{<-c;return "ABC"};func main(){c:=make(chan int);_=scan(input(c))}`, "Error: Deadlock reached"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := fromSource(t, tc.source)
			found := false
			for _, d := range m.Diagnostics {
				if d.Code == "finite-data-loop" {
					found = true
				}
			}
			if !found {
				t.Fatal("rotated proof missing", m.Diagnostics)
			}
			checkTLC(t, m, tc.want)
		})
	}
}
