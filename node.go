package jsonapi

import (
	"fmt"
	"sort"
	"strings"
)

// Payloader is used to encapsulate the One and Many payload types
type Payloader interface {
	clearIncluded()
	filterIncluded(relationshipPaths []string)
}

// OnePayload is used to represent a generic JSON API payload where a single
// resource (Node) was included as an {} in the "data" key
type OnePayload struct {
	Data     *Node   `json:"data"`
	Included []*Node `json:"included,omitempty"`
	Links    *Links  `json:"links,omitempty"`
	Meta     *Meta   `json:"meta,omitempty"`
}

func (p *OnePayload) clearIncluded() {
	p.Included = []*Node{}
}

//TODO see or this can be done cleaner
func (p *OnePayload) filterIncluded(relationshipPaths []string) {
	if p == nil || p.Data == nil || len(p.Included) == 0 {
		return
	}
	allIncludes := make(map[string]*Node, len(p.Included))
	appendNodes(&allIncludes, p.Included...)
	filteredIncludes := make(map[string]*Node, 0)
	for _, path := range relationshipPaths {
		includePath := strings.Split(path, ".")
		oneAppendRelationsToIncludes(&filteredIncludes, p.Data, includePath, allIncludes)
	}
	p.Included = nodeMapValuesSorted(&filteredIncludes)
}

// ManyPayload is used to represent a generic JSON API payload where many
// resources (Nodes) were included in an [] in the "data" key
type ManyPayload struct {
	Data     []*Node `json:"data"`
	Included []*Node `json:"included,omitempty"`
	Links    *Links  `json:"links,omitempty"`
	Meta     *Meta   `json:"meta,omitempty"`
}

func (p *ManyPayload) clearIncluded() {
	p.Included = []*Node{}
}

//TODO see or this can be done cleaner
func (p *ManyPayload) filterIncluded(relationshipPaths []string) {
	if p == nil || len(p.Data) == 0 || len(p.Included) == 0 {
		return
	}
	allIncludes := make(map[string]*Node, len(p.Included))
	appendNodes(&allIncludes, p.Included...)
	filteredIncludes := make(map[string]*Node, 0)
	for _, path := range relationshipPaths {
		relationPath := strings.Split(path, ".")
		manyAppendRelationsToIncludes(&filteredIncludes, p.Data, relationPath, allIncludes)
	}
	p.Included = nodeMapValuesSorted(&filteredIncludes)
}

// Node is used to represent a generic JSON API Resource
type Node struct {
	Type          string                 `json:"type"`
	ID            string                 `json:"id,omitempty"`
	ClientID      string                 `json:"client-id,omitempty"`
	Attributes    map[string]interface{} `json:"attributes,omitempty"`
	Relationships map[string]interface{} `json:"relationships,omitempty"`
	Links         *Links                 `json:"links,omitempty"`
	Meta          *Meta                  `json:"meta,omitempty"`
}

// RelationshipOneNode is used to represent a generic has one JSON API relation
type RelationshipOneNode struct {
	Data  *Node  `json:"data"`
	Links *Links `json:"links,omitempty"`
	Meta  *Meta  `json:"meta,omitempty"`
}

// RelationshipManyNode is used to represent a generic has many JSON API
// relation
type RelationshipManyNode struct {
	Data  []*Node `json:"data"`
	Links *Links  `json:"links,omitempty"`
	Meta  *Meta   `json:"meta,omitempty"`
}

// Links is used to represent a `links` object.
// http://jsonapi.org/format/#document-links
type Links map[string]interface{}

func (l *Links) validate() (err error) {
	// Each member of a links object is a “link”. A link MUST be represented as
	// either:
	//  - a string containing the link’s URL.
	//  - an object (“link object”) which can contain the following members:
	//    - href: a string containing the link’s URL.
	//    - meta: a meta object containing non-standard meta-information about the
	//            link.
	for k, v := range *l {
		_, isString := v.(string)
		_, isLink := v.(Link)

		if !(isString || isLink) {
			return fmt.Errorf(
				"The %s member of the links object was not a string or link object",
				k,
			)
		}
	}
	return
}

// Link is used to represent a member of the `links` object.
type Link struct {
	Href string `json:"href"`
	Meta Meta   `json:"meta,omitempty"`
}

// Linkable is used to include document links in response data
// e.g. {"self": "http://example.com/posts/1"}
type Linkable interface {
	JSONAPILinks() *Links
}

// RelationshipLinkable is used to include relationship links  in response data
// e.g. {"related": "http://example.com/posts/1/comments"}
type RelationshipLinkable interface {
	// JSONAPIRelationshipLinks will be invoked for each relationship with the corresponding relation name (e.g. `comments`)
	JSONAPIRelationshipLinks(relation string) *Links
}

// Meta is used to represent a `meta` object.
// http://jsonapi.org/format/#document-meta
type Meta map[string]interface{}

// Metable is used to include document meta in response data
// e.g. {"foo": "bar"}
type Metable interface {
	JSONAPIMeta() *Meta
}

// RelationshipMetable is used to include relationship meta in response data
type RelationshipMetable interface {
	// JSONRelationshipMeta will be invoked for each relationship with the corresponding relation name (e.g. `comments`)
	JSONAPIRelationshipMeta(relation string) *Meta
}

func manyAppendRelationsToIncludes(includes *map[string]*Node, nodes []*Node, includePath []string, allIncludes map[string]*Node) {
	for _, n := range nodes {
		oneAppendRelationsToIncludes(includes, n, includePath, allIncludes)
	}
}

func oneAppendRelationsToIncludes(includes *map[string]*Node, node *Node, includePath []string, allIncludes map[string]*Node) {
	if len(includePath) < 1 {
		return
	}
	relations := getRelationKeys(node, includePath[0])
	level1Nodes := nodesMapValuesWithKeys(&allIncludes, &relations)
	appendNodes(includes, level1Nodes...)
	if len(includePath) > 1 {
		manyAppendRelationsToIncludes(includes, level1Nodes, includePath[1:], allIncludes)
	}
}

func getRelationKeys(n *Node, relationName string) map[string]bool {
	result := make(map[string]bool, 0)
	if n == nil {
		return result
	}
	relationShips := n.Relationships[relationName]
	if relationShips != nil {
		if r, ok := relationShips.(*RelationshipOneNode); ok && r.Data != nil {
			k := fmt.Sprintf("%s,%s", r.Data.Type, r.Data.ID)
			return map[string]bool{k: true}
		} else if r, ok := relationShips.(*RelationshipManyNode); ok {
			for _, n := range r.Data {
				k := fmt.Sprintf("%s,%s", n.Type, n.ID)
				result[k] = true
			}
		}
	}
	return result
}

func appendNodes(m *map[string]*Node, nodes ...*Node) {
	if m == nil {
		return
	}
	included := *m
	for _, n := range nodes {
		if n == nil {
			continue
		}
		k := fmt.Sprintf("%s,%s", n.Type, n.ID)
		if _, hasNode := included[k]; hasNode {
			continue
		}
		included[k] = n
	}
}

func nodesMapValuesWithKeys(m *map[string]*Node, keys *map[string]bool) []*Node {
	result := make([]*Node, 0)
	for k := range *keys {
		result = append(result, (*m)[k])
	}
	return result
}

func nodeMapValuesSorted(m *map[string]*Node) []*Node {
	nodes := nodeMapValues(m)
	if len(nodes) == 0 {
		return []*Node{}
	}

	// sort by type and id
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Type == nodes[j].Type {
			return nodes[i].ID < nodes[j].ID
		}
		return nodes[i].Type < nodes[j].Type
	})

	return nodes
}
