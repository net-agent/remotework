package kernel_test

import (
	"testing"
	"time"

	"github.com/net-agent/remotework/kernel"
)

// TestBuiltinCapabilities 内核预置能力（EventBus / Catalog）可被插件注入
func TestBuiltinCapabilities(t *testing.T) {
	h := newTestHost(t, nil)

	var bus kernel.EventBus
	var catalog kernel.Catalog
	p := &fakePlugin{
		info: kernel.Info{Name: "consumer"},
		initFn: func(c *kernel.Context) error {
			var err error
			if bus, err = kernel.Inject[kernel.EventBus](c, ""); err != nil {
				return err
			}
			if catalog, err = kernel.Inject[kernel.Catalog](c, ""); err != nil {
				return err
			}
			return nil
		},
	}
	h.Load(p)
	waitState(t, h, "consumer", kernel.StateRunning)

	if bus == nil || catalog == nil {
		t.Fatal("builtin capabilities should be injectable")
	}
	if len(catalog.List()) == 0 {
		t.Fatal("catalog should list loaded plugins")
	}

	// 总线主题过滤：订阅 a，发布 a/b，只收到 a
	ch, unsub := bus.SubscribeTopic(8, "a")
	defer unsub()
	bus.Publish("a", 1)
	bus.Publish("b", 2)
	select {
	case evt := <-ch:
		if evt.Topic != "a" || evt.Payload.(int) != 1 {
			t.Fatalf("unexpected event: %+v", evt)
		}
	case <-time.After(time.Second):
		t.Fatal("expected event on topic a")
	}
}

// TestLookupDynamic Lookup 可在任意时刻查询能力（不产生依赖边）
func TestLookupDynamic(t *testing.T) {
	h := newTestHost(t, nil)

	var ctxRef *kernel.Context
	consumer := &fakePlugin{
		info:   kernel.Info{Name: "consumer"},
		initFn: func(c *kernel.Context) error { ctxRef = c; return nil },
	}
	h.Load(consumer)
	waitState(t, h, "consumer", kernel.StateRunning)

	// 能力未发布：Lookup 失败
	if _, ok := kernel.Lookup[greetCap](ctxRef, "late"); ok {
		t.Fatal("lookup of absent capability should fail")
	}

	// 发布后：Lookup 成功（在回调之外调用）
	h.Load(&fakePlugin{
		info:   kernel.Info{Name: "provider"},
		initFn: func(c *kernel.Context) error { return kernel.Provide[greetCap](c, "late", greetImpl("late")) },
	})
	waitState(t, h, "provider", kernel.StateRunning)

	g, ok := kernel.Lookup[greetCap](ctxRef, "late")
	if !ok || g.Greet() != "late" {
		t.Fatalf("lookup should succeed after provide, ok=%v", ok)
	}

	// 提供者卸载后：Lookup 失败
	if err := h.Unload("provider", kernel.UnloadOptions{}); err != nil {
		t.Fatalf("unload: %v", err)
	}
	if _, ok := kernel.Lookup[greetCap](ctxRef, "late"); ok {
		t.Fatal("lookup should fail after provider unloaded")
	}
}
