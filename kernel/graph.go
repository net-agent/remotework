package kernel

import "sort"

//
// 依赖图：边 = 依赖者 -> 能力提供者。
// 已提交（committed）的依赖图天然是无环的：能力只在插件 Init/Start 成功后提交，
// Inject 只能命中已存在的能力，因此边总是从后装载的插件指向先提交的插件。
// 环只可能出现在"互相等待对方发布能力"的 Pending 插件之间，
// 由 waitFor 图上的 DFS 检测（findCycle）。
//

// resolveOwners 将能力键集合解析为提供者插件名集合
func (h *Host) resolveOwners(deps map[capKey]bool) map[string]bool {
	owners := make(map[string]bool, len(deps))
	for k := range deps {
		if e, ok := h.caps.get(k); ok {
			owners[e.owner] = true
		}
	}
	return owners
}

// dependentsOf 返回当前仍引用 name 所提供能力的插件名（不含己卸载/卸载中）
func (h *Host) dependentsOf(name string) []string {
	var out []string
	for _, m := range h.nodes {
		if m.info.Name == name {
			continue
		}
		if m.state == StateStopping || m.state == StateUnloaded {
			continue
		}
		if m.depProviders[name] {
			out = append(out, m.info.Name)
		}
	}
	sort.Strings(out)
	return out
}

// transitiveDependents 返回 name 的全部传递依赖者（用于级联卸载标记）
func (h *Host) transitiveDependents(name string) []string {
	seen := map[string]bool{}
	queue := []string{name}
	var out []string
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, dep := range h.dependentsOf(cur) {
			if seen[dep] {
				continue
			}
			seen[dep] = true
			out = append(out, dep)
			queue = append(queue, dep)
		}
	}
	sort.Strings(out)
	return out
}

// hasDependentsInRegistry 是否仍有插件引用 name 的能力。
// 与 dependentsOf 不同，这里不豁免 Stopping：
// 用于卸载排序——依赖者必须彻底移除后，被依赖者才能开始清理。
func (h *Host) hasDependentsInRegistry(n *node) bool {
	for _, m := range h.nodes {
		if m.info.Name == n.info.Name {
			continue
		}
		if m.depProviders[n.info.Name] {
			return true
		}
	}
	return false
}

// waitFor 等待图：n 缺失的能力 -> 可能提供它的插件（已提交的提供者，或曾尝试提供的插件）
func (h *Host) waitFor(n *node) map[string]bool {
	out := map[string]bool{}
	for k := range n.missing {
		if e, ok := h.caps.get(k); ok {
			if e.owner != n.info.Name {
				out[e.owner] = true
			}
			continue
		}
		for _, m := range h.nodes {
			if m.info.Name == n.info.Name {
				continue
			}
			if containsKey(m.attemptedProvides, k) {
				out[m.info.Name] = true
			}
		}
	}
	return out
}

// findCycle 在等待图上从 startName 做 DFS，返回包含 startName 的环路径（无环返回 nil）
func (h *Host) findCycle(startName string) []string {
	var path []string
	onStack := map[string]bool{}
	visited := map[string]bool{}

	var visit func(name string) []string
	visit = func(name string) []string {
		if onStack[name] {
			for i, p := range path {
				if p == name {
					return append([]string(nil), path[i:]...)
				}
			}
			return []string{name}
		}
		if visited[name] {
			return nil
		}
		visited[name] = true
		n := h.nodes[name]
		if n == nil {
			return nil
		}
		onStack[name] = true
		path = append(path, name)
		defer func() {
			path = path[:len(path)-1]
			onStack[name] = false
		}()
		for next := range h.waitFor(n) {
			if cycle := visit(next); cycle != nil {
				return cycle
			}
		}
		return nil
	}

	return visit(startName)
}

func containsKey(keys []capKey, k capKey) bool {
	for _, key := range keys {
		if key == k {
			return true
		}
	}
	return false
}

func keysOf(provides map[capKey]any) []capKey {
	if len(provides) == 0 {
		return nil
	}
	keys := make([]capKey, 0, len(provides))
	for k := range provides {
		keys = append(keys, k)
	}
	return keys
}
