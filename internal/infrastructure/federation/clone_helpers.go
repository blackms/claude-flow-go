package federation

import (
	"reflect"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

func cloneSwarmRegistration(swarm *shared.SwarmRegistration) *shared.SwarmRegistration {
	if swarm == nil {
		return nil
	}

	cloned := *swarm
	cloned.Capabilities = append([]string(nil), swarm.Capabilities...)
	cloned.Metadata = cloneStringInterfaceMap(swarm.Metadata)
	return &cloned
}

func cloneEphemeralAgent(agent *shared.EphemeralAgent) *shared.EphemeralAgent {
	if agent == nil {
		return nil
	}

	cloned := *agent
	cloned.Metadata = cloneStringInterfaceMap(agent.Metadata)
	cloned.Result = cloneInterfaceValue(agent.Result)
	return &cloned
}

func cloneFederationMessage(message *shared.FederationMessage) *shared.FederationMessage {
	if message == nil {
		return nil
	}

	cloned := *message
	cloned.Payload = cloneInterfaceValue(message.Payload)
	return &cloned
}

func cloneFederationProposal(proposal *shared.FederationProposal) *shared.FederationProposal {
	if proposal == nil {
		return nil
	}

	cloned := *proposal
	cloned.Value = cloneInterfaceValue(proposal.Value)
	if proposal.Votes != nil {
		cloned.Votes = make(map[string]bool, len(proposal.Votes))
		for voterID, approve := range proposal.Votes {
			cloned.Votes[voterID] = approve
		}
	}
	return &cloned
}

func cloneFederationEvent(event *shared.FederationEvent) *shared.FederationEvent {
	if event == nil {
		return nil
	}

	cloned := *event
	cloned.Data = cloneInterfaceValue(event.Data)
	return &cloned
}

func cloneStringInterfaceMap(source map[string]interface{}) map[string]interface{} {
	if source == nil {
		return nil
	}

	cloned := make(map[string]interface{}, len(source))
	for key, value := range source {
		cloned[key] = cloneInterfaceValue(value)
	}
	return cloned
}

func cloneInterfaceValue(value interface{}) interface{} {
	if value == nil {
		return nil
	}
	return cloneReflectValue(reflect.ValueOf(value)).Interface()
}

func cloneReflectValue(value reflect.Value) reflect.Value {
	if !value.IsValid() {
		return value
	}

	switch value.Kind() {
	case reflect.Map:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		clonedMap := reflect.MakeMapWithSize(value.Type(), value.Len())
		for _, key := range value.MapKeys() {
			clonedMap.SetMapIndex(key, cloneReflectValue(value.MapIndex(key)))
		}
		return clonedMap

	case reflect.Slice:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		clonedSlice := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for i := 0; i < value.Len(); i++ {
			clonedSlice.Index(i).Set(cloneReflectValue(value.Index(i)))
		}
		return clonedSlice

	case reflect.Array:
		clonedArray := reflect.New(value.Type()).Elem()
		for i := 0; i < value.Len(); i++ {
			clonedArray.Index(i).Set(cloneReflectValue(value.Index(i)))
		}
		return clonedArray

	case reflect.Ptr:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		clonedPointer := reflect.New(value.Type().Elem())
		clonedPointer.Elem().Set(cloneReflectValue(value.Elem()))
		return clonedPointer

	case reflect.Interface:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		return cloneReflectValue(value.Elem())

	case reflect.Struct:
		clonedStruct := reflect.New(value.Type()).Elem()
		clonedStruct.Set(value)
		for i := 0; i < value.NumField(); i++ {
			destinationField := clonedStruct.Field(i)
			if !destinationField.CanSet() {
				continue
			}

			clonedField := cloneReflectValue(value.Field(i))
			if !clonedField.IsValid() {
				continue
			}

			if clonedField.Type().AssignableTo(destinationField.Type()) {
				destinationField.Set(clonedField)
			} else if clonedField.Type().ConvertibleTo(destinationField.Type()) {
				destinationField.Set(clonedField.Convert(destinationField.Type()))
			}
		}
		return clonedStruct

	default:
		return value
	}
}
