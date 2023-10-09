package construct2

import (
	"errors"

	"github.com/dominikbraun/graph"
	"github.com/klothoplatform/klotho/pkg/set"
)

func copyVertexProps(p graph.VertexProperties) func(*graph.VertexProperties) {
	return func(dst *graph.VertexProperties) {
		*dst = p
	}
}

func copyEdgeProps(p graph.EdgeProperties) func(*graph.EdgeProperties) {
	return func(dst *graph.EdgeProperties) {
		*dst = p
	}
}

// ReplaceResource replaces the resources identified by `oldId` with `newRes` in the graph and in any property
// references (as [ResourceId] or [PropertyRef]) of the old ID to the new ID in any resource that depends on or is
// depended on by the resource.
func ReplaceResource(g Graph, oldId ResourceId, newRes *Resource) error {
	r, props, err := g.VertexWithProperties(oldId)
	if err != nil {
		return err
	}

	err = g.AddVertex(r, copyVertexProps(props))
	if err != nil {
		return err
	}

	neighbors := make(set.Set[ResourceId])
	adj, err := g.AdjacencyMap()
	if err != nil {
		return err
	}
	for _, edge := range adj[oldId] {
		err = errors.Join(
			err,
			g.AddEdge(r.ID, edge.Target, copyEdgeProps(edge.Properties)),
			g.RemoveEdge(edge.Source, edge.Target),
		)
		neighbors.Add(edge.Target)
	}
	if err != nil {
		return err
	}

	pred, err := g.PredecessorMap()
	if err != nil {
		return err
	}
	for _, edge := range pred[oldId] {
		err = errors.Join(
			err,
			g.AddEdge(edge.Source, r.ID, copyEdgeProps(edge.Properties)),
			g.RemoveEdge(edge.Source, edge.Target),
		)
		neighbors.Add(edge.Source)
	}
	if err != nil {
		return err
	}

	if err := g.RemoveVertex(oldId); err != nil {
		return err
	}

	updateId := func(path PropertyPathItem) error {
		itemVal := path.Get()
		itemId, ok := itemVal.(ResourceId)
		if ok && itemId == oldId {
			return path.Set(r.ID)
		}
		itemRef, ok := itemVal.(PropertyRef)
		if ok && itemRef.Resource == oldId {
			itemRef.Resource = r.ID
			return path.Set(itemRef)
		}
		return nil
	}

	for neighborId := range neighbors {
		neighbor, err := g.Vertex(neighborId)
		if err != nil {
			return err
		}
		err = neighbor.WalkProperties(func(path PropertyPath, err error) error {
			err = errors.Join(err, updateId(path))
			kv, ok := path.Last().(PropertyKVItem)
			if !ok {
				return err
			}
			err = errors.Join(err, updateId(kv.Key()))
			return err
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// UpdateResourceId is used when a resource's ID changes. It updates the graph in-place, using the resource
// currently referenced by `old`. No-op if the resource ID hasn't changed.
// Also updates any property references (as [ResourceId] or [PropertyRef]) of the old ID to the new ID in any
// resource that depends on or is depended on by the resource.
func PropagateUpdatedId(g Graph, old ResourceId) error {
	newRes, err := g.Vertex(old)
	if err != nil {
		return err
	}
	// Short circuit if the resource ID hasn't changed.
	if old == newRes.ID {
		return nil
	}
	return ReplaceResource(g, old, newRes)
}

// RemoveResource removes all edges from the resource. any property references (as [ResourceId] or [PropertyRef])
// to the resource, and finally the resource itself.
func RemoveResource(g Graph, id ResourceId) error {
	neighbors := make(map[ResourceId]struct{})
	adj, err := g.AdjacencyMap()
	if err != nil {
		return err
	}
	if _, ok := adj[id]; !ok {
		return nil
	}

	for _, edge := range adj[id] {
		err = errors.Join(
			err,
			g.RemoveEdge(edge.Source, edge.Target),
		)
		neighbors[edge.Target] = struct{}{}
	}
	if err != nil {
		return err
	}

	pred, err := g.PredecessorMap()
	if err != nil {
		return err
	}
	for _, edge := range pred[id] {
		err = errors.Join(
			err,
			g.RemoveEdge(edge.Source, edge.Target),
		)
		neighbors[edge.Source] = struct{}{}
	}
	if err != nil {
		return err
	}

	removeId := func(path PropertyPathItem) error {
		itemVal := path.Get()
		itemId, ok := itemVal.(ResourceId)
		if ok && itemId == id {
			return path.Remove(nil)
		}
		itemRef, ok := itemVal.(PropertyRef)
		if ok && itemRef.Resource == id {
			return path.Remove(nil)
		}
		return nil
	}

	for neighborId := range neighbors {
		neighbor, err := g.Vertex(neighborId)
		if err != nil {
			return err
		}
		err = neighbor.WalkProperties(func(path PropertyPath, err error) error {
			err = errors.Join(err, removeId(path))
			kv, ok := path.Last().(PropertyKVItem)
			if !ok {
				return err
			}
			err = errors.Join(err, removeId(kv.Key()))
			return err
		})
		if err != nil {
			return err
		}
	}
	return g.RemoveVertex(id)
}
