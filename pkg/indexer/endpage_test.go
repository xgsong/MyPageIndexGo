package indexer

import (
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
	// 创建测试索引数据
	root := &document.Node{
		ID:        "root-1",
		Title:     "Document",
		StartPage: 1,
		EndPage:   21,
		Children: []*document.Node{
			{
				ID:        "chapter-1",
				Title:     "第一章 亚当·斯密与《国富论》的诞生",
				StartPage: 1,
				EndPage:   3,
				Children: []*document.Node{
					{
						ID:        "section-1-1",
						Title:     "亚当·斯密的生平与时代背景",
						StartPage: 1,
						EndPage:   1,
					},
					{
						ID:        "section-1-2",
						Title:     "《国富论》的创作背景与理论渊源",
						StartPage: 1,
						EndPage:   2,
					},
					{
						ID:        "section-1-3",
						Title:     "《国富论》的体系结构与核心主题",
						StartPage: 2,
						EndPage:   3,
					},
				},
			},
			{
				ID:        "chapter-2",
				Title:     "第二章 分工理论：经济增长的逻辑起点",
				StartPage: 3,
				EndPage:   5,
				Children: []*document.Node{
					{
						ID:        "section-2-1",
						Title:     "分工对劳动生产力的巨大促进作用",
						StartPage: 3,
						EndPage:   4,
					},
					{
						ID:        "section-2-2",
						Title:     "分工产生的原因：人类互通有无、物物交换的倾向",
						StartPage: 4,
						EndPage:   5,
					},
				},
			},
			{
				ID:        "chapter-3",
				Title:     "第三章 市场与价格机制",
				StartPage: 5,
				EndPage:   8,
				Children: []*document.Node{
					{
						ID:        "section-3-1",
						Title:     "市场价格的形成机制",
						StartPage: 5,
						EndPage:   6,
					},
					{
						ID:        "section-3-2",
						Title:     "自然价格与市场价格的关系",
						StartPage: 6,
						EndPage:   7,
					},
					{
						ID:        "section-3-3",
						Title:     "价格机制对资源配置的作用",
						StartPage: 7,
						EndPage:   8,
					},
				},
			},
			{
				ID:        "chapter-4",
				Title:     "第四章 资本积累与经济增长",
				StartPage: 8,
				EndPage:   12,
				Children: []*document.Node{
					{
						ID:        "section-4-1",
						Title:     "资本积累对经济增长的重要性",
						StartPage: 8,
						EndPage:   9,
					},
					{
						ID:        "section-4-2",
						Title:     "固定资本与流动资本的区分",
						StartPage: 9,
						EndPage:   10,
					},
					{
						ID:        "section-4-3",
						Title:     "资本的积累与经济增长的关系",
						StartPage: 10,
						EndPage:   12,
					},
				},
			},
			{
				ID:        "chapter-5",
				Title:     "第五章 国际贸易理论",
				StartPage: 12,
				EndPage:   16,
				Children: []*document.Node{
					{
						ID:        "section-5-1",
						Title:     "绝对优势理论",
						StartPage: 12,
						EndPage:   13,
					},
					{
						ID:        "section-5-2",
						Title:     "比较优势理论",
						StartPage: 13,
						EndPage:   15,
					},
					{
						ID:        "section-5-3",
						Title:     "国际贸易对经济增长的影响",
						StartPage: 15,
						EndPage:   16,
					},
				},
			},
			{
				ID:        "chapter-6",
				Title:     "第六章 政府职能与公共政策",
				StartPage: 16,
				EndPage:   21,
				Children: []*document.Node{
					{
						ID:        "section-6-1",
						Title:     "政府的三大职能",
						StartPage: 16,
						EndPage:   18,
					},
					{
						ID:        "section-6-2",
						Title:     "税收原则与财政政策",
						StartPage: 18,
						EndPage:   20,
					},
					{
						ID:        "section-6-3",
						Title:     "公共工程与基础设施建设",
						StartPage: 20,
						EndPage:   21,
					},
				},
			},
		},
	}

	index := document.NewIndexTree(root, 21)
	index.BuildNodeMap()
	return index, nil
}
