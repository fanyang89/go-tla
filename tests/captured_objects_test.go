package tests

import (
	"testing"

	"github.com/fanmi/go-tla/internal/tla"
)

func TestCapturedSyncObjectRefusals(t *testing.T) {
	for name, body := range map[string]string{
		"future-store":      `var p *box;go func(){p.mu.Lock()}();p=&box{}`,
		"reassignment":      `p:=&box{};go func(){p.mu.Lock()}();p=&box{}`,
		"closure-write":     `p:=&box{};go func(){p=&box{};p.mu.Lock()}()`,
		"nested-write":      `p:=&box{};go func(){go func(){p=&box{};p.mu.Lock()}()}()`,
		"escaped-cell":      `p:=&box{};change(&p);go func(){p.mu.Lock()}()`,
		"conditional-store": `var p *box;if unknown(){p=&box{}};go func(){p.mu.Lock()}()`,
		"nil":               `var p *box;go func(){p.mu.Lock()}()`,
		"whole-copy":        `p:=&box{};go func(){copy:=*p;copy.mu.Lock()}()`,
	} {
		t.Run(name, func(t *testing.T) {
			m := fromSource(t, `package main;import "sync";type box struct{mu sync.Mutex};func unknown()bool;func change(p **box){*p=&box{}};func main(){`+body+`}`, "fixture.unknown")
			if !m.HasErrors() {
				t.Fatal("unproved pointer-cell capture accepted")
			}
			if _, _, err := tla.Generate(m); err == nil {
				t.Fatal("unsafe capture executable")
			}
		})
	}
}

func TestTLCCapturedSyncObjects(t *testing.T) {
	for name, body := range map[string]string{
		"shared":   `p:=&box{};p.mu.Lock();done:=make(chan int);go func(){p.mu.Unlock();done<-1}();<-done`,
		"distinct": `a:=&box{};b:=&box{};a.mu.Lock();b.mu.Lock();done:=make(chan int);go func(){a.mu.Unlock();b.mu.Unlock();done<-1}();<-done`,
	} {
		t.Run(name, func(t *testing.T) {
			m := fromSource(t, `package main;import "sync";type box struct{mu sync.Mutex};func main(){`+body+`}`)
			checkTLC(t, m, "No error has been found")
		})
	}
}
