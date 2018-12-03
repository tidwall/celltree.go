// Copyright 2018 Joshua J Baker. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package celltree

const maxItems = 256 // max items per node
const nBits = 8      // 1, 2,  4,   8  - match nNodes with the correct nBits
const nNodes = 256   // 2, 4, 16, 256  - match nNodes with the correct nBits

type item struct {
	cell uint64
	data interface{}
}

type node struct {
	branch bool    // is a branch (not a leaf)
	ncount byte    // tracks non-nil nodes, max is 256
	items  []item  // leaf items
	nodes  []*node // child nodes
}

// Tree is a uint64 prefix tree
type Tree struct {
	count int   // number of items in tree
	root  *node // root node
}

// Insert inserts an item into the tree. Items are ordered by it's cell.
// The extra param is a simple user context value.
func (tr *Tree) Insert(cell uint64, data interface{}) {
	if tr.root == nil {
		tr.root = new(node)
	}
	tr.insert(tr.root, cell, data, 64-nBits)
	tr.count++
}

// Count returns the number of items in the tree.
func (tr *Tree) Count() int {
	return tr.count
}

func cellIndex(cell uint64, bits uint) int {
	return int(cell >> bits & uint64(nNodes-1))
}

func (tr *Tree) insert(n *node, cell uint64, data interface{}, bits uint) {
	if !n.branch {
		// leaf node
		if bits != 0 && len(n.items) >= maxItems {
			// split leaf. it's at capacity
			tr.split(n, bits)
			// insert item again
			tr.insert(n, cell, data, bits)
		} else {
			// find the target index for the new cell
			if len(n.items) == 0 || n.items[len(n.items)-1].cell <= cell {
				// the new cell is greater than the last cell in leaf, so
				// we can just append it
				n.items = append(n.items, item{cell: cell, data: data})
			} else {
				index := tr.find(n, cell)
				// create space for the new cell
				n.items = append(n.items, item{})
				// move other cells over to make room for new cell
				copy(n.items[index+1:], n.items[index:len(n.items)-1])
				// assign the new cell
				n.items[index] = item{cell: cell, data: data}
			}
		}
	} else {
		// branch node
		index := cellIndex(cell, bits)
		for index >= len(n.nodes) {
			n.nodes = append(n.nodes, nil)
		}
		if n.nodes[index] == nil {
			n.nodes[index] = new(node)
			n.ncount++
		}
		tr.insert(n.nodes[index], cell, data, bits-nBits)
	}
}

func (tr *Tree) split(n *node, bits uint) {
	n.branch = true
	for i := 0; i < len(n.items); i++ {
		tr.insert(n, n.items[i].cell, n.items[i].data, bits)
	}
	n.items = nil
}

// find an index of the cell using a binary search
func (tr *Tree) find(n *node, cell uint64) int {
	i, j := 0, len(n.items)
	for i < j {
		h := i + (j-i)/2
		if !(cell < n.items[h].cell) {
			i = h + 1
		} else {
			j = h
		}
	}
	return i
}

// Remove removes an item from the tree based on it's cell and data values.
func (tr *Tree) Remove(cell uint64, data interface{}) {
	if tr.root == nil {
		return
	}
	if tr.remove(tr.root, cell, data, 64-nBits, nil) {
		tr.count--
	}
}

func (tr *Tree) remove(n *node, cell uint64, data interface{}, bits uint,
	cond func(data interface{}) bool,
) bool {
	if !n.branch {
		i := tr.find(n, cell) - 1
		for ; i >= 0; i-- {
			if n.items[i].cell != cell {
				break
			}
			if (cond == nil && n.items[i].data == data) ||
				(cond != nil && cond(n.items[i].data)) {
				n.items[i] = item{}
				copy(n.items[i:len(n.items)-1], n.items[i+1:])
				n.items = n.items[:len(n.items)-1]
				return true
			}
		}
		return false
	}
	index := cellIndex(cell, bits)
	if index >= len(n.nodes) || n.nodes[index] == nil ||
		!tr.remove(n.nodes[index], cell, data, bits-nBits, cond) {
		// didn't find the cell
		return false
	}
	if !n.nodes[index].branch && len(n.nodes[index].items) == 0 {
		// target leaf is empty, remove it.
		n.nodes[index] = nil
		n.ncount--
		if n.ncount == 0 {
			// node is empty, convert it to a leaf
			n.branch = false
			n.items = nil
		}
	}
	return true
}

// RemoveWhen removes an item from the tree based on it's cell and
// when the cond func returns true. It will delete at most a maximum of one item.
func (tr *Tree) RemoveWhen(cell uint64, cond func(data interface{}) bool) {
	if tr.root == nil {
		return
	}
	if tr.remove(tr.root, cell, nil, 64-nBits, cond) {
		tr.count--
	}
}

// Scan iterates over the entire tree. Return false from the iter function to stop.
func (tr *Tree) Scan(iter func(cell uint64, data interface{}) bool) {
	if tr.root == nil {
		return
	}
	tr.scan(tr.root, iter)
}

func (tr *Tree) scan(n *node, iter func(cell uint64, data interface{}) bool) bool {
	if !n.branch {
		for i := 0; i < len(n.items); i++ {
			if !iter(n.items[i].cell, n.items[i].data) {
				return false
			}
		}
	} else {
		for i := 0; i < len(n.nodes); i++ {
			if n.nodes[i] != nil {
				if !tr.scan(n.nodes[i], iter) {
					return false
				}
			}
		}
	}
	return true
}

// Range iterates over the three start with the cell param.
func (tr *Tree) Range(cell uint64, iter func(cell uint64, key interface{}) bool) {
	if tr.root == nil {
		return
	}
	tr._range(tr.root, cell, 64-nBits, iter)
}

func (tr *Tree) _range(n *node, cell uint64, bits uint, iter func(cell uint64, data interface{}) bool) (hit, ok bool) {
	if !n.branch {
		hit = true
		i := tr.find(n, cell) - 1
		for ; i >= 0; i-- {
			if n.items[i].cell < cell {
				break
			}
		}
		i++
		for ; i < len(n.items); i++ {
			if !iter(n.items[i].cell, n.items[i].data) {
				return hit, false
			}
		}
		return hit, true
	}
	index := cellIndex(cell, bits)
	if index >= len(n.nodes) || n.nodes[index] == nil {
		return hit, true
	}
	for ; index < len(n.nodes); index++ {
		if n.nodes[index] != nil {
			if hit {
				if !tr.scan(n.nodes[index], iter) {
					return hit, false
				}
			} else {
				hit, ok = tr._range(n.nodes[index], cell, bits-nBits, iter)
				if !ok {
					return hit, false
				}
			}
		}
	}
	return hit, true
}
