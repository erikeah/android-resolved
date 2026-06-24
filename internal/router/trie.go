package router

import (
	"strings"
	"sync"

	"github.com/anomalyco/android-resolved/internal/config"
)

type trieNode struct {
	rule  *config.Rule
	child map[string]*trieNode
}

type TrieRouter struct {
	mu    sync.RWMutex
	root  *trieNode
	rules []config.Rule
}

func NewTrieRouter(rules []config.Rule) *TrieRouter {
	tr := &TrieRouter{root: &trieNode{child: make(map[string]*trieNode)}}
	tr.Update(rules)
	return tr
}

func (tr *TrieRouter) Route(domain string) *Match {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	labels := reverseLabels(domain)
	node := tr.root
	var best *trieNode

	for _, label := range labels {
		child, ok := node.child[label]
		if !ok {
			child, ok = node.child["*"]
			if !ok {
				break
			}
		}
		node = child
		if node.rule != nil {
			best = node
		}
	}

	if best == nil {
		if node.rule != nil {
			best = node
		}
		if best == nil {
			if catchAll, ok := tr.root.child["."]; ok && catchAll.rule != nil {
				best = catchAll
			}
		}
	}

	if best == nil || best.rule == nil {
		return nil
	}

	return &Match{
		Rule:     best.rule,
		Upstream: best.rule.Upstream,
	}
}

func (tr *TrieRouter) Rules() []config.Rule {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	return tr.rules
}

func (tr *TrieRouter) Update(rules []config.Rule) {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	tr.root = &trieNode{child: make(map[string]*trieNode)}
	tr.rules = make([]config.Rule, len(rules))
	copy(tr.rules, rules)

	for i := range rules {
		rule := &rules[i]
		domain := rule.Domain
		if domain == "." {
			tr.root.child["."] = &trieNode{rule: rule}
			continue
		}
		labels := reverseLabels(domain)
		node := tr.root
		for _, label := range labels {
			if node.child == nil {
				node.child = make(map[string]*trieNode)
			}
			next, ok := node.child[label]
			if !ok {
				next = &trieNode{child: make(map[string]*trieNode)}
				node.child[label] = next
			}
			node = next
		}
		node.rule = rule
	}
}

func reverseLabels(domain string) []string {
	domain = strings.TrimSuffix(domain, ".")
	if domain == "" {
		return nil
	}
	parts := strings.Split(domain, ".")
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	return parts
}
