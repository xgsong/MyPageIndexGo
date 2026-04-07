package indexer

import (
	"regexp"
	"strings"

	"github.com/xgsong/mypageindexgo/pkg/document"
)

func mergeDuplicateChapters(rootNodes []*document.Node, nodes map[string]*document.Node) ([]*document.Node, map[string]*document.Node) {
	chapterTitleToNode := make(map[string]*document.Node)
	for _, node := range rootNodes {
		if matches := regexp.MustCompile(`第(.+?)章`).FindStringSubmatch(node.Title); len(matches) > 1 {
			chapterNum := strings.TrimSpace(matches[1])
			if allArabic := func(s string) bool {
				for _, r := range s {
					if r < '0' || r > '9' {
						return false
					}
				}
				return true
			}(chapterNum); allArabic {
				chapterNum = normalizeArabicToChinese(chapterNum)
			}

			if existing, ok := chapterTitleToNode[chapterNum]; ok {
				if len(node.Title) > len(existing.Title) {
					chapterTitleToNode[chapterNum] = node
				}
			} else {
				chapterTitleToNode[chapterNum] = node
			}
		}
	}

	deduplicatedRoots := make([]*document.Node, 0, len(rootNodes))
	skippedNodes := make(map[*document.Node]bool)

	for _, node := range rootNodes {
		if skippedNodes[node] {
			continue
		}

		if matches := regexp.MustCompile(`第(.+?)章`).FindStringSubmatch(node.Title); len(matches) > 1 {
			chapterNum := strings.TrimSpace(matches[1])
			if allArabic := func(s string) bool {
				for _, r := range s {
					if r < '0' || r > '9' {
						return false
					}
				}
				return true
			}(chapterNum); allArabic {
				chapterNum = normalizeArabicToChinese(chapterNum)
			}

			preferredNode := chapterTitleToNode[chapterNum]
			if preferredNode != node {
				for _, child := range node.Children {
					preferredNode.AddChild(child)
				}
				if node.StartPage < preferredNode.StartPage {
					preferredNode.StartPage = node.StartPage
				}
				if node.EndPage > preferredNode.EndPage {
					preferredNode.EndPage = node.EndPage
				}
				skippedNodes[node] = true
				continue
			}
		}

		deduplicatedRoots = append(deduplicatedRoots, node)
	}

	return deduplicatedRoots, nodes
}
