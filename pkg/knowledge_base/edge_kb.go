package knowledgebase

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/klothoplatform/klotho/pkg/core"
	"github.com/klothoplatform/klotho/pkg/graph"
	"go.uber.org/zap"
)

type (
	// Edge defines an entry in a Knowledge base. An Edge represents a valid linking between two types of resources
	Edge struct {
		// Source represents the source resource in the edge
		Source reflect.Type
		// Destination represents the target resource in the edge
		Destination reflect.Type
	}

	// EdgeDetails defines the set of characteristics and edge in the knowledge base contains. The details are used to ensure graph correctness for ResourceGraphs
	EdgeDetails struct {
		// Configure is a function used to configure the To and From resources and necessary dependent resources, to ensure the nodes will guarantee correct functionality.
		Configure ConfigureEdge
		// DirectEdgeOnly signals that the edge cannot be used within constructing other paths and can only be used as a direct edge
		DirectEdgeOnly bool
		// DeploymentOrderReversed is specified when the edge is in the opposite direction of the deployment order
		DeploymentOrderReversed bool
		// DeletetionDependent is used to specify edges which should not influence the deletion criteria of a resource
		// a true value specifies the target being deleted is dependent on the source and do not need to depend on satisfication of the deletion criteria to attempt to delete the true source of the edge.
		DeletetionDependent bool
		//Reuse tells us whether we can reuse an upstream or downstream resource during path selection and node creation
		Reuse Reuse
	}

	// Reuse is set to represent an enum of possible reuse cases for edges. The current available options are upstream and downstream
	Reuse string

	// EdgeKB is a map (knowledge base) of edges and their respective details used to configure ResourceGraphs
	EdgeKB map[Edge]EdgeDetails

	// EdgeConfigurer is a function used to configure the To and From resources and necessary dependent resources, to ensure the nodes will guarantee correct functionality.
	ConfigureEdge func(source, dest core.Resource, dag *core.ResourceGraph, data EdgeData) error

	// EdgeConstraint is an object defined on EdgeData which can influence the path picked when expansion occurs.
	EdgeConstraint struct {
		// NodeMustExist specifies a list of resources which must exist in the path when edge expansion occurs. The resources type will be correlated to the types in the generated paths
		NodeMustExist []core.Resource
		// NodeMustNotExist specifies a list of resources which must not exist when edge expansion occurs. The resources type will be correlated to the types in the generated paths
		NodeMustNotExist []core.Resource
	}

	// EdgeData is an object attached to edges in the ResourceGraph to help the knowledge base understand context when performing expansion and configuration tasks
	EdgeData struct {
		// AppName refers to the application name of the global ResourceGraph
		AppName string
		// EnvironmentVaribles specify and environment variables which will need to be configured during the edge expansion process
		EnvironmentVariables []core.EnvironmentVariable
		// Constraint refers to the EdgeConstraints defined during the edge expansion
		Constraint EdgeConstraint
		// Source refers to the initial source resource node when edge expansion is called
		Source core.Resource
		// Destination refers to the initial target resource node when edge expansion is called
		Destination core.Resource
		// Attributes is a map of attributes which can be used to store arbitrary data on the edge
		Attributes map[string]any
	}

	Path []Edge
)

const (
	Upstream   Reuse = "upstream"
	Downstream Reuse = "downstream"
)

func MergeKBs(kbsToUse []EdgeKB) (EdgeKB, error) {
	kb := EdgeKB{}
	var err error
	for _, currKb := range kbsToUse {
		for edge, detail := range currKb {
			if _, found := kb[edge]; found {
				err = errors.Join(err, fmt.Errorf("edge for %s -> %s is already defined in the knowledge base", edge.Source, edge.Destination))
			}
			kb[edge] = detail
		}
	}
	return kb, err
}

func NewEdge[Src core.Resource, Dest core.Resource]() Edge {
	var src Src
	var dest Dest
	return Edge{Source: reflect.TypeOf(src), Destination: reflect.TypeOf(dest)}
}

// GetEdge takes in a source and target to retrieve the edge details for the given key. Will return nil if no edge exists for the given source and target
func (kb EdgeKB) GetEdge(source core.Resource, target core.Resource) (EdgeDetails, bool) {
	return kb.GetEdgeDetails(reflect.TypeOf(source), reflect.TypeOf(target))
}

// GetEdgeDetails takes in a source and target to retrieve the edge details for the given key. Will return nil if no edge exists for the given source and target
func (kb EdgeKB) GetEdgeDetails(source reflect.Type, target reflect.Type) (EdgeDetails, bool) {
	detail, found := kb[Edge{Source: source, Destination: target}]
	return detail, found
}

// GetEdgesWithSource will return all edges where the source type parameter is the From of the edge
func (kb EdgeKB) GetEdgesWithSource(source reflect.Type) []Edge {
	result := []Edge{}
	for edge := range kb {
		if edge.Source == source {
			result = append(result, edge)
		}
	}
	return result
}

// GetEdgesWithTarget will return all edges where the target type parameter is the To of the edge
func (kb EdgeKB) GetEdgesWithTarget(target reflect.Type) []Edge {
	result := []Edge{}
	for edge := range kb {
		if edge.Destination == target {
			result = append(result, edge)
		}
	}
	return result
}

// FindPaths takes in a source and destination type and finds all valid paths to get from source to destination.
//
// Find paths does a Depth First Search to search through all edges in the knowledge base.
// The function tracks visited edges to prevent cycles during execution
// It also checks the ValidDestinations for each edge against the original destination node to ensure that the edge is allowed to be used in the instance of the path generation
//
// The method will return all paths found
func (kb EdgeKB) FindPaths(source core.Resource, dest core.Resource, constraint EdgeConstraint) []Path {
	visitedEdges := map[reflect.Type]bool{}
	stack := []Edge{}
	paths := kb.findPaths(reflect.TypeOf(source), reflect.TypeOf(dest), stack, visitedEdges)
	validPaths := []Path{}
	for _, path := range paths {
		// Ensure that the path satisfies the NodeMustExist edge constraint
		if constraint.NodeMustExist != nil {
			nodeFound := false
			for _, res := range path {
				for _, mustExistRes := range constraint.NodeMustExist {
					if res.Source == reflect.TypeOf(mustExistRes) || res.Destination == reflect.TypeOf(mustExistRes) {
						nodeFound = true
					}
				}
			}
			if !nodeFound {
				continue
			}
		}

		// Ensure that the path satisfies the NodeMustNotExist edge constraint
		if constraint.NodeMustNotExist != nil {
			nodeFound := false
			for _, res := range path {
				for _, mustNotExistRes := range constraint.NodeMustNotExist {
					if res.Source == reflect.TypeOf(mustNotExistRes) || res.Destination == reflect.TypeOf(mustNotExistRes) {
						nodeFound = true
					}
				}
			}
			if nodeFound {
				continue
			}
		}
		validPaths = append(validPaths, path)
	}
	return validPaths
}

// findPaths performs the recursive calls of the parent FindPath function
//
// It works under the assumption that an edge is bidirectional and uses the edges ValidDestinations field to determine when that assumption is incorrect
func (kb EdgeKB) findPaths(source reflect.Type, dest reflect.Type, stack []Edge, visited map[reflect.Type]bool) []Path {
	visited[source] = true
	var result []Path

	if source == dest {
		// For resources which can have dependencies between themselves we have to add that path to the stack if it is a valid edge
		if len(stack) == 0 {
			if _, found := kb.GetEdgeDetails(source, dest); found {
				stack = append(stack, Edge{Source: source, Destination: dest})
			}
		}
		if len(stack) != 0 {
			var clonedStack []Edge
			clonedStack = append(clonedStack, stack...)
			result = append(result, clonedStack)
		}
	} else {

		// When we are not at the destination we want to recursively call findPaths on all edges which have the source as the current node
		// This is checking all edges which have a direction of From -> To
		for _, e := range kb.GetEdgesWithSource(source) {
			det, _ := kb.GetEdgeDetails(e.Source, e.Destination)
			// Ensure that direct edges cannot contribute to paths. We check if its a direct match for the dest and if not we continue
			if det.DirectEdgeOnly && (len(stack) != 0 || e.Destination != dest) {
				continue
			}
			if e.Source == source && !visited[e.Destination] {
				result = append(result, kb.findPaths(e.Destination, dest, append(stack, e), visited)...)
			}
		}
	}
	delete(visited, source)
	return result
}

// ExpandEdges performs calculations to determine the proper path to be inserted into the ResourceGraph.
//
// The workflow of the edge expansion is as follows:
//   - Find shortest path given the constraints on the edge
//   - Iterate through each edge in path creating the resource if necessary
func (kb EdgeKB) ExpandEdge(dep *graph.Edge[core.Resource], dag *core.ResourceGraph, validPath Path, edgeData EdgeData) (err error) {

	// most likely need to use downstream and upstream operational errors here

	// It does not matter what order we go in as each edge should be expanded independently. They can still reuse resources since the create methods should be idempotent if resources are the same.
	zap.S().Debugf("Expanding Edge for %s -> %s", dep.Source.Id(), dep.Destination.Id())

	resourceCache := map[reflect.Type]core.Resource{}
	name := fmt.Sprintf("%s_%s", dep.Source.Id().Name, dep.Destination.Id().Name)
	for _, edge := range validPath {
		source := edge.Source
		dest := edge.Destination
		edgeDetail, _ := kb.GetEdgeDetails(source, dest)
		sourceNode := resourceCache[source]
		// Determine if the source node is the actual source of the dependency getting expanded
		if source == reflect.TypeOf(dep.Source) {
			sourceNode = dep.Source
		}
		if sourceNode == nil {
			// Create a new interface of the source nodes type if it does not exist
			sourceNode = reflect.New(source.Elem()).Interface().(core.Resource)
			reflect.ValueOf(sourceNode).Elem().FieldByName("Name").Set(reflect.ValueOf(fmt.Sprintf("%s_%s", sourceNode.Id().Type, name)))
			// Determine if the source node is the same type as what is specified in the constraints as must exist
			for _, mustExistRes := range edgeData.Constraint.NodeMustExist {
				if mustExistRes.Id().Type == sourceNode.Id().Type && mustExistRes.Id().Provider == sourceNode.Id().Provider && mustExistRes.Id().Namespace == sourceNode.Id().Namespace {
					sourceNode = mustExistRes
				}
			}
		}

		// Determine if the destination node is the actual destination of the dependency getting expanded
		destNode := resourceCache[dest]
		if dest == reflect.TypeOf(dep.Destination) {
			destNode = dep.Destination
		}

		if destNode == nil {
			// Create a new interface of the destination nodes type if it does not exist
			destNode = reflect.New(dest.Elem()).Interface().(core.Resource)
			reflect.ValueOf(destNode).Elem().FieldByName("Name").Set(reflect.ValueOf(fmt.Sprintf("%s_%s", destNode.Id().Type, name)))
			// Determine if the destination node is the same type as what is specified in the constraints as must exist
			for _, mustExistRes := range edgeData.Constraint.NodeMustExist {
				if mustExistRes.Id().Type == destNode.Id().Type && mustExistRes.Id().Provider == destNode.Id().Provider && mustExistRes.Id().Namespace == destNode.Id().Namespace {
					destNode = mustExistRes
				}
			}
		}

		added := false

		// If the edge specifies that it can reuse upstream or downstream resources, we want to find the first resource which satisfies the reuse criteria and add that as the dependency.
		// If there is no resource that satisfies the reuse criteria, we want to add the original direct dependency
		switch edgeDetail.Reuse {
		case Upstream:
			upstreamResources := dag.GetAllDownstreamResources(dep.Source)
			for _, res := range upstreamResources {
				if sourceNode.Id().Type == res.Id().Type {
					dag.AddDependencyWithData(res, destNode, EdgeData{Source: dep.Source, Destination: dep.Destination})
					added = true
				}
			}
		case Downstream:
			upstreamResources := dag.GetAllDownstreamResources(dep.Destination)
			for _, res := range upstreamResources {
				if destNode.Id().Type == res.Id().Type {
					dag.AddDependencyWithData(sourceNode, res, EdgeData{Source: dep.Source, Destination: dep.Destination})
					added = true
				}
			}
		}
		if added {
			break
		}

		dag.AddDependencyWithData(sourceNode, destNode, EdgeData{Source: dep.Source, Destination: dep.Destination})

		if sourceNode != nil {
			resourceCache[source] = sourceNode
		}
		sourceNodeInGraph := dag.GetResource(sourceNode.Id())
		if sourceNodeInGraph != nil {
			resourceCache[source] = sourceNodeInGraph
		}
		if destNode != nil {
			resourceCache[dest] = destNode
		}
		destNodeInGraph := dag.GetResource(destNode.Id())
		if destNodeInGraph != nil {
			resourceCache[dest] = destNodeInGraph
		}
	}

	// If the valid path is not the original direct path, we want to remove the initial direct dependency so we can fill in the new edges with intermediate nodes
	if len(validPath) > 1 {
		zap.S().Debugf("Removing dependency from %s -> %s", dep.Source.Id(), dep.Destination.Id())
		err := dag.RemoveDependency(dep.Source.Id(), dep.Destination.Id())
		if err != nil {
			return err
		}

	}
	return nil
}

// ConfigureEdge calls each edge configure function.
func (kb EdgeKB) ConfigureEdge(dep *graph.Edge[core.Resource], dag *core.ResourceGraph) (err error) {
	zap.S().Debugf("Configuring Edge for %s -> %s", dep.Source.Id(), dep.Destination.Id())
	source := reflect.TypeOf(dep.Source)
	destination := reflect.TypeOf(dep.Destination)
	edgeData := EdgeData{}
	data, ok := dep.Properties.Data.(EdgeData)
	if !ok && dep.Properties.Data != nil {
		return fmt.Errorf("edge properties for edge %s -> %s, do not satisfy edge data format during edge configuration", dep.Source.Id(), dep.Destination.Id())
	} else if dep.Properties.Data != nil {
		edgeData = data
	}
	edgeDetail, found := kb.GetEdgeDetails(source, destination)
	if !found {
		return fmt.Errorf("internal error invalid edge for edge %s -> %s (no such edge in Edge KB)", dep.Source.Id(), dep.Destination.Id())
	}
	if edgeDetail.Configure != nil {
		err := edgeDetail.Configure(dep.Source, dep.Destination, dag, edgeData)
		if err != nil {
			return err
		}
	}
	return nil
}
