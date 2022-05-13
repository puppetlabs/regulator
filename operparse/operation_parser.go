package operparse

import (
	"fmt"

	"github.com/puppetlabs/regulator/defaultimpls"
	"github.com/puppetlabs/regulator/operation"
	"github.com/puppetlabs/regulator/rgerror"
	"gopkg.in/yaml.v2"
)

// Idempotent function for merging new data in to Operations
// struct. Can be used more than once to read data from multiple
// sources
func ParseOperations(raw_data []byte, data *operation.Operations) *rgerror.RGerror {
	unmarshald_data := operation.Operations{}
	err := yaml.UnmarshalStrict(raw_data, &unmarshald_data)
	if err != nil {
		return &rgerror.RGerror{
			Kind:    rgerror.ExecError,
			Message: fmt.Sprintf("Failed to parse yaml:\n%s", err),
			Origin:  err,
		}
	}
	rgerr := ConcatOperations(data, &unmarshald_data)
	if rgerr != nil {
		return rgerr
	}
	return nil
}

// Yeah this is big and ugly and could probably have helper functions,
// but I don't want to do that much interface magic and pass enough
// strings around to make the messages different and helpful.
func ConcatOperations(first *operation.Operations, second *operation.Operations) *rgerror.RGerror {
	var conflicts map[string]string = make(map[string]string)
	if first.Observations == nil {
		first.Observations = make(map[string]operation.Observation)
	}
	if first.Reactions == nil {
		first.Reactions = make(map[string]operation.Reaction)
	}
	if first.Actions == nil {
		first.Actions = make(map[string]operation.Action)
	}
	if first.Implements == nil {
		first.Implements = make(map[string]operation.Implement)
	}
	for obsv_name, obsv := range second.Observations {
		if obsv.Empty() {
			return &rgerror.RGerror{
				Kind:    rgerror.InvalidInput,
				Message: fmt.Sprintf("Observation '%s' is empty, observations must have all of 'entity', 'query', and 'instance' set", obsv_name),
				Origin:  nil,
			}
		}
		for _, key := range obsv.HashKeys() {
			if conflict, conflicted := conflicts[key]; conflicted == true {
				// When observations have a collision that's not necessarily
				// a conflict, we have to check if the expect field is different.
				//
				// If the field _is_ different then there is a conflict, otherwise
				// it's fine. In the case where they are the same we don't need to
				// add this latest observation to the conflicts map because
				// there's already a matching hash there
				if first.Observations[conflict].Expect != obsv.Expect {
					return &rgerror.RGerror{
						Kind:    rgerror.InvalidInput,
						Message: fmt.Sprintf("Observation '%s' conflicts with '%s'", obsv_name, conflict),
						Origin:  nil,
					}
				}
			} else {
				conflicts[key] = obsv_name
			}
		}
		first.Observations[obsv_name] = obsv
	}
	for rctn_name, rctn := range second.Reactions {
		if rctn.Empty() {
			return &rgerror.RGerror{
				Kind:    rgerror.InvalidInput,
				Message: fmt.Sprintf("Reaction '%s' is empty, reactions must have all of 'observation', 'action', and 'condition check/value' set", rctn_name),
				Origin:  nil,
			}
		}
		for _, key := range rctn.HashKeys() {
			if conflict, conflicted := conflicts[key]; conflicted == true {
				return &rgerror.RGerror{
					Kind:    rgerror.InvalidInput,
					Message: fmt.Sprintf("Reaction '%s' conflicts with '%s'", rctn_name, conflict),
					Origin:  nil,
				}
			} else {
				conflicts[key] = rctn_name
			}
		}
		first.Reactions[rctn_name] = rctn
	}
	for actn_name, actn := range second.Actions {
		if actn.Empty() {
			return &rgerror.RGerror{
				Kind:    rgerror.InvalidInput,
				Message: fmt.Sprintf("Action '%s' is empty, actions must have 'exe' and one of 'path' or 'script' set", actn_name),
				Origin:  nil,
			}
		}
		for _, key := range actn.HashKeys() {
			if conflict, conflicted := conflicts[key]; conflicted == true {
				return &rgerror.RGerror{
					Kind:    rgerror.InvalidInput,
					Message: fmt.Sprintf("Action '%s' conflicts with '%s'", actn_name, conflict),
					Origin:  nil,
				}
			} else {
				conflicts[key] = actn_name
			}
		}
		first.Actions[actn_name] = actn
	}
	// We have to edit the impls in the second operations to ensure
	// that the default impls don't create conflicts
	impls_to_add := second.Implements
	// Ensure that the default impls are added first so that
	// any attempts to add an impl with the same name as a
	// default will always conflict.
	for default_impl_name, default_impl := range defaultimpls.DEFAULT_IMPLS {
		// Don't even check for collisions or anything, just re-add
		// all the defaults every time.
		first.Implements[default_impl_name] = default_impl
		for _, key := range default_impl.HashKeys() {
			conflicts[key] = default_impl_name
		}
		// Remove all the default impls from the impls that are to be
		// added to the first operations.
		delete(impls_to_add, default_impl_name)
	}
	for impl_name, impl := range impls_to_add {
		if impl.Empty() {
			return &rgerror.RGerror{
				Kind:    rgerror.InvalidInput,
				Message: fmt.Sprintf("Implement '%s' is empty, implements must have 'exe' set, one of 'path' or 'script' set, and either react or observe or both", impl_name),
				Origin:  nil,
			}
		}
		for _, key := range impl.HashKeys() {
			if conflict, conflicted := conflicts[key]; conflicted == true {
				return &rgerror.RGerror{
					Kind:    rgerror.InvalidInput,
					Message: fmt.Sprintf("Implement '%s' conflicts with '%s'", impl_name, conflict),
					Origin:  nil,
				}
			} else {
				conflicts[key] = impl_name
			}
		}
		first.Implements[impl_name] = impl
	}
	return nil
}

// Replaces a special string in a list of arguments (used for observations and
// reaction impls) with specific data from elsewhere
func ComputeArgs(arg_spec []string, obsv operation.Observation) []string {
	var args []string
	for _, a := range arg_spec {
		switch a {
		case "instance":
			args = append(args, obsv.Instance)
		default:
			args = append(args, a)
		}
	}
	return args
}

func SelectAction(actn_name string, actns map[string]operation.Action) *operation.Action {
	if selected_action, found := actns[actn_name]; found {
		return &selected_action
	}
	return nil
}

func SelectObservation(obsv_name string, obsvs map[string]operation.Observation) *operation.Observation {
	if selected_obs, found := obsvs[obsv_name]; found {
		return &selected_obs
	}
	return nil
}

func SelectObservationResult(obsv_name string, obsv_results map[string]operation.ObservationResult) *operation.ObservationResult {
	if selected_obsv_result, found := obsv_results[obsv_name]; found {
		return &selected_obsv_result
	}
	return nil
}

func SelectImplementActionByName(impl_name string, impls map[string]operation.Implement) *operation.Action {
	if selected_impl, found := impls[impl_name]; found {
		return &operation.Action{
			Path: selected_impl.Path,
			Exe:  selected_impl.Exe,
			Args: selected_impl.Reacts.Args,
		}
	}
	return nil
}

func SelectImplementActionForCorrection(obsv operation.Observation, obsv_result operation.ObservationResult, impls map[string]operation.Implement) (string, *operation.Action) {
	for impl_name, impl := range impls {
		if impl.Reacts.Corrects.Entity == obsv.Entity &&
			impl.Reacts.Corrects.Query == obsv.Query &&
			impl.Reacts.Corrects.Results_In == obsv.Expect {
			for _, state := range impl.Reacts.Corrects.Starts_From {
				if state == obsv_result.Result {
					return impl_name, &operation.Action{
						Path: impl.Path,
						Exe:  impl.Exe,
						Args: impl.Reacts.Args,
					}
				}
			}
		}
	}
	return "", nil
}
