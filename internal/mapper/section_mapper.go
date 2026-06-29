package mapper

import "github.com/atlas/knowledge-api/internal/domain"

type SectionResponse struct {
	ID       string            `json:"id"`
	Title    string            `json:"title"`
	Content  string            `json:"content"`
	Children []SectionResponse `json:"children,omitempty"`
}

func BuildSectionTree(sections []domain.Section) []SectionResponse {
	byParent := make(map[string][]domain.Section)
	roots := make([]domain.Section, 0)

	for _, s := range sections {
		if s.ParentID == nil {
			roots = append(roots, s)
			continue
		}
		byParent[*s.ParentID] = append(byParent[*s.ParentID], s)
	}

	var build func(domain.Section) SectionResponse
	build = func(s domain.Section) SectionResponse {
		resp := SectionResponse{ID: s.ID, Title: s.Title, Content: s.Content}
		children := byParent[s.ID]
		if len(children) > 0 {
			resp.Children = make([]SectionResponse, 0, len(children))
			for _, child := range children {
				resp.Children = append(resp.Children, build(child))
			}
		}
		return resp
	}

	result := make([]SectionResponse, 0, len(roots))
	for _, root := range roots {
		result = append(result, build(root))
	}
	return result
}
