package runner

import (
	"reflect"
	"testing"
)

func TestWatchedItems(t *testing.T) {
	items := []string{"夜叉书匣", "铁剑令", "金蛇藏宝图", "铁剑令"}
	got := watchedItems(items, []string{" 铁剑令"})
	want := []string{"铁剑令"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("watchedItems()=%v, want %v", got, want)
	}
}

func TestWatchedItemsEmpty(t *testing.T) {
	if got := watchedItems([]string{"夜叉书匣"}, []string{"铁剑令"}); got != nil {
		t.Fatalf("watchedItems()=%v, want nil", got)
	}
}

func TestSourceStateKey(t *testing.T) {
	if got, want := sourceStateKey(noticeShopSource, "h5_1"), "notice_shop:h5_1"; got != want {
		t.Fatalf("sourceStateKey()=%q, want %q", got, want)
	}
}
