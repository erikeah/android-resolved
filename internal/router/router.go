package router

import "github.com/anomalyco/android-resolved/internal/config"

type Match struct {
	Rule     *config.Rule
	Upstream string
}

type Router interface {
	Route(domain string) *Match
	Rules() []config.Rule
	Update(rules []config.Rule)
}
