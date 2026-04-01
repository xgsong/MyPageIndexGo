package indexer

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xgsong/mypageindexgo/pkg/document"
)

func TestEndPageCalculation_BasicVerification(t *testing.T) {
	index, err := loadTestIndex()
	require.NoError(t, err, "加载测试索引失败")

	var validateNode func(node *document.Node) error
	validateNode = func(node *document.Node) error {
		if node.EndPage < node.StartPage {
			return assert.AnError
		}
		for _, child := range node.Children {
			if err := validateNode(child); err != nil {
				return err
			}
		}
		return nil
	}

	err = validateNode(index.Root)
	assert.NoError(t, err, "所有节点的 EndPage 必须 >= StartPage")

	var validateParentChildRanges func(node *document.Node) error
	validateParentChildRanges = func(node *document.Node) error {
		if len(node.Children) == 0 {
			return nil
		}

		minChildStart := node.Children[0].StartPage
		maxChildEnd := node.Children[0].EndPage

		for _, child := range node.Children {
			if child.StartPage < minChildStart {
				minChildStart = child.StartPage
			}
			if child.EndPage > maxChildEnd {
				maxChildEnd = child.EndPage
			}
		}

		if node.StartPage != minChildStart {
			t.Errorf("父节点 %s 的 StartPage(%d) 应等于子节点最小 StartPage(%d)",
				node.Title, node.StartPage, minChildStart)
		}
		if node.EndPage != maxChildEnd {
			t.Errorf("父节点 %s 的 EndPage(%d) 应等于子节点最大 EndPage(%d)",
				node.Title, node.EndPage, maxChildEnd)
		}

		for _, child := range node.Children {
			if err := validateParentChildRanges(child); err != nil {
				return err
			}
		}
		return nil
	}

	err = validateParentChildRanges(index.Root)
	assert.NoError(t, err, "父子节点页码范围不一致")
}

func TestEndPageCalculation_SameStartPageSiblings(t *testing.T) {
	index, err := loadTestIndex()
	require.NoError(t, err)

	var findSameStartPageSiblings func(node *document.Node)
	findSameStartPageSiblings = func(node *document.Node) {
		if len(node.Children) < 2 {
			return
		}

		startPageMap := make(map[int][]*document.Node)
		for _, child := range node.Children {
			startPageMap[child.StartPage] = append(startPageMap[child.StartPage], child)
		}

		for _, sameStartSiblings := range startPageMap {
			if len(sameStartSiblings) < 2 {
				continue
			}

			for i := 0; i < len(sameStartSiblings)-1; i++ {
				currentNode := sameStartSiblings[i]
				nextNode := sameStartSiblings[i+1]

				if currentNode.StartPage == currentNode.EndPage {
					continue
				}
				assert.LessOrEqual(t, currentNode.EndPage, nextNode.StartPage,
					"同起始页节点 %s 和 %s: 前一个节点的 EndPage(%d) 应 <= 后一个节点的 StartPage(%d)",
					currentNode.Title, nextNode.Title, currentNode.EndPage, nextNode.StartPage)
			}
		}

		for _, child := range node.Children {
			findSameStartPageSiblings(child)
		}
	}

	findSameStartPageSiblings(index.Root)
}

func TestEndPageCalculation_NoGaps(t *testing.T) {
	index, err := loadTestIndex()
	require.NoError(t, err)

	var checkSiblingGaps func(node *document.Node)
	checkSiblingGaps = func(node *document.Node) {
		if len(node.Children) < 2 {
			return
		}

		sorted := make([]*document.Node, len(node.Children))
		copy(sorted, node.Children)
		for i := 0; i < len(sorted)-1; i++ {
			for j := i + 1; j < len(sorted); j++ {
				if sorted[i].StartPage > sorted[j].StartPage {
					sorted[i], sorted[j] = sorted[j], sorted[i]
				}
			}
		}

		for i := 0; i < len(sorted)-1; i++ {
			currentEnd := sorted[i].EndPage
			nextStart := sorted[i+1].StartPage

			if currentEnd < nextStart-1 {
				t.Errorf("兄弟节点 %s 和 %s 之间存在页码间隙：[%d-%d] 和 [%d-%d]",
					sorted[i].Title, sorted[i+1].Title,
					sorted[i].StartPage, sorted[i].EndPage,
					sorted[i+1].StartPage, sorted[i+1].EndPage)
			}
		}

		for _, child := range node.Children {
			checkSiblingGaps(child)
		}
	}

	checkSiblingGaps(index.Root)
}

func TestEndPageCalculation_EdgeCases(t *testing.T) {
	testCases := []struct {
		name             string
		physicalIndices  []int
		expectedEndPages []int
	}{
		{
			name:             "单节点",
			physicalIndices:  []int{1},
			expectedEndPages: []int{1},
		},
		{
			name:             "连续不同页",
			physicalIndices:  []int{1, 2, 3},
			expectedEndPages: []int{1, 2, 3},
		},
		{
			name:             "同页多节点",
			physicalIndices:  []int{1, 1, 1},
			expectedEndPages: []int{1, 1, 1},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			items := make([]struct {
				Name          string
				PhysicalIndex int
				StartPage     int
				EndPage       int
			}, len(tc.physicalIndices))

			for i, physIdx := range tc.physicalIndices {
				items[i] = struct {
					Name          string
					PhysicalIndex int
					StartPage     int
					EndPage       int
				}{
					Name:          tc.name,
					PhysicalIndex: physIdx,
					StartPage:     physIdx,
					EndPage:       physIdx,
				}
			}

			calculateEndPage(items)

			for i, expectedEnd := range tc.expectedEndPages {
				assert.Equal(t, expectedEnd, items[i].EndPage,
					"节点 %d 的 EndPage 不匹配：期望 %d, 实际 %d", i, expectedEnd, items[i].EndPage)
			}
		})
	}
}

func calculateEndPage(items []struct {
	Name          string
	PhysicalIndex int
	StartPage     int
	EndPage       int
}) {
	for i := 0; i < len(items); i++ {
		nextDifferentPage := items[i].StartPage + 1
		samePageNext := false

		for j := i + 1; j < len(items); j++ {
			if items[j].PhysicalIndex != items[i].PhysicalIndex {
				nextDifferentPage = items[j].StartPage
				break
			}
			if j == i+1 {
				samePageNext = true
			}
		}

		if nextDifferentPage == items[i].StartPage+1 {
			items[i].EndPage = items[i].StartPage
		} else {
			if samePageNext {
				items[i].EndPage = items[i].StartPage
			} else {
				items[i].EndPage = nextDifferentPage - 1
			}
		}
	}
}

func TestRecalculatePageRanges(t *testing.T) {
	index, err := loadTestIndex()
	require.NoError(t, err)

	var validateParentRanges func(node *document.Node) error
	validateParentRanges = func(node *document.Node) error {
		if len(node.Children) == 0 {
			return nil
		}

		minStart := node.Children[0].StartPage
		maxEnd := node.Children[0].EndPage
		for _, child := range node.Children[1:] {
			if child.StartPage < minStart {
				minStart = child.StartPage
			}
			if child.EndPage > maxEnd {
				maxEnd = child.EndPage
			}
		}

		if node.StartPage != minStart {
			t.Errorf("父节点 %s 的 StartPage(%d) 应等于子节点最小 StartPage(%d)",
				node.Title, node.StartPage, minStart)
		}
		if node.EndPage != maxEnd {
			t.Errorf("父节点 %s 的 EndPage(%d) 应等于子节点最大 EndPage(%d)",
				node.Title, node.EndPage, maxEnd)
		}

		for _, child := range node.Children {
			if err := validateParentRanges(child); err != nil {
				return err
			}
		}
		return nil
	}

	err = validateParentRanges(index.Root)
	assert.NoError(t, err, "父节点页码范围未正确更新")
}

func loadTestIndex() (*document.IndexTree, error) {
	testIndexPath := "/tmp/test_endpage.json"
	data, err := os.ReadFile(testIndexPath)
	if err != nil {
		return nil, err
	}

	var index document.IndexTree
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, err
	}

	index.BuildNodeMap()
	return &index, nil
}
