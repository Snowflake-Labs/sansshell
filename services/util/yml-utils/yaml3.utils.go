/* Copyright (c) 2024 Snowflake Inc. All rights reserved.

   Licensed under the Apache License, Version 2.0 (the
   "License"); you may not use this file except in compliance
   with the License.  You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing,
   software distributed under the License is distributed on an
   "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
   KIND, either express or implied.  See the License for the
   specific language governing permissions and limitations
   under the License.
*/

package yml_utils

import (
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
	"strconv"
	"strings"
)

// Yaml3PathNode is a node of YmlPath
type Yaml3PathNode interface {
	GetPrev() Yaml3PathNode
	SetPrev(prev Yaml3PathNode)
	GetNext() Yaml3PathNode
	SetNext(next Yaml3PathNode)
	GetScalarValueFrom(node *yaml.Node) (string, error)
	SetScalarValueTo(node *yaml.Node, value string, style yaml.Style) error
}

//////////////////////////////////////////////////////////////////////////////////////
// Implementation of [yaml3PathGetFromRoot]
//////////////////////////////////////////////////////////////////////////////////////

// yaml3PathGetFromRoot root node of YmlPath, implementation of [Yaml3PathNode] interface
// e.g. for path "$.items[0]" it would be representation of "$"
type yaml3PathGetFromRoot struct {
	next Yaml3PathNode
}

// GetScalarValueFrom implements [Yaml3PathNode.GetScalarValueFrom] interface
func (p *yaml3PathGetFromRoot) GetScalarValueFrom(node *yaml.Node) (string, error) {
	if p.next == nil {
		return "", fmt.Errorf("%s could not get scalar of root", getPathToNode(p))
	}

	childNode, err := p.getRelevantNode(node)
	if err != nil {
		return "", err
	}

	return p.next.GetScalarValueFrom(childNode)
}

// SetScalarValueTo implements [Yaml3PathNode.SetScalarValueTo] interface
func (p *yaml3PathGetFromRoot) SetScalarValueTo(node *yaml.Node, value string, style yaml.Style) error {
	childNode, err := p.getRelevantNode(node)
	if err != nil {
		return err
	}

	if p.next == nil {
		return fmt.Errorf("%s could not set scalar of root", getPathToNode(p))
	}

	return p.next.SetScalarValueTo(childNode, value, style)
}

// SetNext implements [Yaml3PathNode.SetNext] interface
func (p *yaml3PathGetFromRoot) SetNext(next Yaml3PathNode) {
	p.next = next
	if next.GetPrev() != p {
		next.SetPrev(p)
	}
}

// SetPrev implements [Yaml3PathNode.SetPrev] interface
func (p *yaml3PathGetFromRoot) SetPrev(prev Yaml3PathNode) {
	panic("could not set prev for root")
}

// GetPrev implements [Yaml3PathNode.GetPrev] interface
func (p *yaml3PathGetFromRoot) GetPrev() Yaml3PathNode {
	return nil
}

// GetNext implements [Yaml3PathNode.GetNext] interface
func (p *yaml3PathGetFromRoot) GetNext() Yaml3PathNode {
	return p.next
}

func (p *yaml3PathGetFromRoot) getRelevantNode(node *yaml.Node) (*yaml.Node, error) {
	if node.Kind != yaml.DocumentNode {
		return nil, fmt.Errorf("%s document node is expected", getPathToNode(p))
	}

	if len(node.Content) == 0 {
		return nil, fmt.Errorf("%s could not get value from empty root", getPathToNode(p))
	}

	return node.Content[0], nil
}

//////////////////////////////////////////////////////////////////////////////////////
// Implementation of [yaml3PathGetByIndex]
//////////////////////////////////////////////////////////////////////////////////////

// yaml3PathGetByIndex get item from sequence node of YmlPath, implementation of [Yaml3PathNode] interface
// e.g. for path "$.items[0]" it would be representation of "[0]"
type yaml3PathGetByIndex struct {
	next  Yaml3PathNode
	prev  Yaml3PathNode
	index int
}

// GetScalarValueFrom implements [Yaml3PathNode.GetScalarValueFrom] interface
func (p *yaml3PathGetByIndex) GetScalarValueFrom(node *yaml.Node) (string, error) {
	childNode, err := p.getRelevantNode(node)
	if err != nil {
		return "", err
	}

	if p.next != nil {
		return p.next.GetScalarValueFrom(childNode)
	}

	if childNode.Kind != yaml.ScalarNode {
		return "", fmt.Errorf("%s scalar node is expected, but found %s", getPathToNode(p), kindToString(childNode))
	}

	return childNode.Value, nil
}

// SetScalarValueTo implements [Yaml3PathNode.SetScalarValueTo] interface
func (p *yaml3PathGetByIndex) SetScalarValueTo(node *yaml.Node, value string, style yaml.Style) error {
	childNode, err := p.getRelevantNode(node)
	if err != nil {
		return err
	}

	if p.next != nil {
		return p.next.SetScalarValueTo(childNode, value, style)
	}

	if childNode.Kind != yaml.ScalarNode {
		return fmt.Errorf("%s scalar node is expected, but found %s", getPathToNode(p), kindToString(childNode))
	}

	childNode.Value = value
	childNode.Style = style
	return nil
}

// SetNext implements [Yaml3PathNode.SetNext] interface
func (p *yaml3PathGetByIndex) SetNext(next Yaml3PathNode) {
	p.next = next
	if next.GetPrev() != p {
		next.SetPrev(p)
	}
}

// SetPrev implements [Yaml3PathNode.SetPrev] interface
func (p *yaml3PathGetByIndex) SetPrev(prev Yaml3PathNode) {
	p.prev = prev
	if prev.GetNext() != p {
		prev.SetNext(p)
	}
}

// GetPrev implements [Yaml3PathNode.GetPrev] interface
func (p *yaml3PathGetByIndex) GetPrev() Yaml3PathNode {
	return p.prev
}

// GetNext implements [Yaml3PathNode.GetNext] interface
func (p *yaml3PathGetByIndex) GetNext() Yaml3PathNode {
	return p.next
}

func (p *yaml3PathGetByIndex) getRelevantNode(node *yaml.Node) (*yaml.Node, error) {
	if node.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("%s sequence node expected, but found %s", getPathToNode(p.GetPrev()), kindToString(node))
	}

	if p.index >= len(node.Content) {
		return nil, fmt.Errorf("%s index out of sequence range", getPathToNode(p))
	}

	childNode := node.Content[p.index]
	return childNode, nil
}

//////////////////////////////////////////////////////////////////////////////////////
// Implementation of [yaml3PathGetByKey]
//////////////////////////////////////////////////////////////////////////////////////

// yaml3PathGetByKey get item from mapping node of YmlPath, implementation of [Yaml3PathNode] interface
// e.g. for path "$.items[0]" it would be representation of ".items"
type yaml3PathGetByKey struct {
	next Yaml3PathNode
	prev Yaml3PathNode
	key  string
}

// GetScalarValueFrom implements [Yaml3PathNode.GetScalarValueFrom] interface
func (p *yaml3PathGetByKey) GetScalarValueFrom(node *yaml.Node) (string, error) {
	nestedNode, err := p.getRelevantNode(node)
	if err != nil {
		return "", err
	}

	if p.next != nil {
		return p.next.GetScalarValueFrom(nestedNode)
	}

	if nestedNode.Kind != yaml.ScalarNode {
		return "", fmt.Errorf("%s scalar node is expected, but found %s", getPathToNode(p), kindToString(node))
	}

	return nestedNode.Value, nil
}

// SetScalarValueTo implements [Yaml3PathNode.SetScalarValueTo] interface
func (p *yaml3PathGetByKey) SetScalarValueTo(node *yaml.Node, value string, style yaml.Style) error {
	nestedNode, err := p.getRelevantNode(node)
	if err != nil {
		return err
	}

	if p.next != nil {
		return p.next.SetScalarValueTo(nestedNode, value, style)
	}

	if nestedNode.Kind != yaml.ScalarNode {
		return fmt.Errorf("%s scalar node is expected, but found %s", getPathToNode(p), kindToString(node))
	}

	nestedNode.Value = value
	nestedNode.Style = style

	return nil
}

// SetNext implements [Yaml3PathNode.SetNext] interface
func (p *yaml3PathGetByKey) SetNext(next Yaml3PathNode) {
	p.next = next
	if next.GetPrev() != p {
		next.SetPrev(p)
	}
}

// SetPrev implements [Yaml3PathNode.SetPrev] interface
func (p *yaml3PathGetByKey) SetPrev(prev Yaml3PathNode) {
	p.prev = prev
	if prev.GetNext() != p {
		prev.SetNext(p)
	}
}

// GetPrev implements [Yaml3PathNode.GetPrev] interface
func (p *yaml3PathGetByKey) GetPrev() Yaml3PathNode {
	return p.prev
}

// GetNext implements [Yaml3PathNode.GetNext] interface
func (p *yaml3PathGetByKey) GetNext() Yaml3PathNode {
	return p.next
}

func (p *yaml3PathGetByKey) getRelevantNode(node *yaml.Node) (*yaml.Node, error) {
	if node.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s mapping node expected, but found %s", getPathToNode(p.GetPrev()), kindToString(node))
	}

	nodeByKeyIndex := -1
	for index, childNode := range node.Content {
		if childNode.Value == p.key {
			nodeByKeyIndex = index
			break
		}
	}

	if nodeByKeyIndex == -1 {
		return nil, fmt.Errorf("%s there is no value by key", getPathToNode(p))
	}

	if nodeByKeyIndex+1 >= len(node.Content) {
		// in yaml3 mapping or scalar value node is next to node found by key
		return nil, fmt.Errorf("%s there is no value by key", getPathToNode(p))
	}
	nestedNode := node.Content[nodeByKeyIndex+1]

	return nestedNode, nil
}

//////////////////////////////////////////////////////////////////////////////////////
// Public functions
//////////////////////////////////////////////////////////////////////////////////////

// ParseYmlPath parses yaml path string and returns path node
func ParseYmlPath(path string) (Yaml3PathNode, error) {
	artifacts := strings.Split(path, ".")
	if len(artifacts) == 0 {
		return nil, errors.New("empty path")
	}

	if artifacts[0] != "$" {
		return nil, errors.New("path not start from root")
	}

	var root Yaml3PathNode = &yaml3PathGetFromRoot{}

	node := root
	artifacts = artifacts[1:]
	for i, artifact := range artifacts {
		artifact = strings.TrimSpace(artifact)
		keyWithIndex := strings.Split(artifact, "[")

		if len(keyWithIndex) > 2 {
			fullKey := strings.Join(artifacts[:i+1], ".")
			return nil, fmt.Errorf("%s invalid path, too many \"[\" in key", fullKey)
		}

		key := keyWithIndex[0]
		byKeyGetPath := &yaml3PathGetByKey{
			key: key,
		}
		node.SetNext(byKeyGetPath)
		node = byKeyGetPath

		if len(keyWithIndex) == 2 {
			indexArtifact := keyWithIndex[1]
			if indexArtifact[len(indexArtifact)-1] != ']' {
				fullKey := strings.Join(artifacts[:i+1], ".")
				return nil, fmt.Errorf("%s invalid path, sequence index has no \"]\"", fullKey)
			}

			indexStr := indexArtifact[:len(indexArtifact)-1]
			seqIndex, err := strconv.Atoi(indexStr)
			if err != nil {
				fullKey := strings.Join(artifacts[:i+1], ".")
				return nil, fmt.Errorf("%s invalid sequence index", fullKey)
			}
			byIndexGetPath := &yaml3PathGetByIndex{
				index: seqIndex,
			}
			node.SetNext(byIndexGetPath)
			node = byIndexGetPath
		}
	}

	return root, nil
}

//////////////////////////////////////////////////////////////////////////////////////
// Private functions
//////////////////////////////////////////////////////////////////////////////////////

// getPathRoot returns root of provided path
func getPathRoot(path Yaml3PathNode) Yaml3PathNode {
	var root = path
	for root.GetPrev() != nil {
		root = root.GetPrev()
	}

	return root
}

// getPathToNode returns string representation of path to provided path node
// e.g.
// if you have parsed "$.issues.exclude-rules[0].text[0]" path, and you provide "text" path node
// this function will return "$.issues.exclude-rules[0].text"
func getPathToNode(path Yaml3PathNode) string {
	var root = getPathRoot(path)

	current := root
	builder := strings.Builder{}
	for current != nil {
		switch v := current.(type) {
		case *yaml3PathGetFromRoot:
			builder.WriteString("$")
		case *yaml3PathGetByKey:
			builder.WriteString(".")
			builder.WriteString(v.key)
		case *yaml3PathGetByIndex:
			builder.WriteString("[")
			builder.WriteString(strconv.Itoa(v.index))
			builder.WriteString("]")
		default:
			panic(fmt.Sprintf("unknown path node type: %v", current))
		}

		if current == path {
			break
		}

		current = current.GetNext()
	}

	return builder.String()
}

var kindMap = map[yaml.Kind]string{
	yaml.DocumentNode: "document node",
	yaml.SequenceNode: "sequence node",
	yaml.MappingNode:  "mapping node",
	yaml.ScalarNode:   "scalar node",
	yaml.AliasNode:    "alias node",
}

func kindToString(node *yaml.Node) string {
	// kind is in map
	if kindName, ok := kindMap[node.Kind]; ok {
		return kindName
	}

	panic(fmt.Sprintf("unknown kind: %v", node.Kind))
}
