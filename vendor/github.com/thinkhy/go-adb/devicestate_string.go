// Code generated by "stringer -type=DeviceState"; DO NOT EDIT

package adb

import "fmt"

const _DeviceState_name = "StateInvalidStateUnauthorizedStateDisconnectedStateOfflineStateOnline"

var _DeviceState_index = [...]uint8{0, 12, 29, 46, 58, 69}

func (i DeviceState) String() string {
	if i < 0 || i >= DeviceState(len(_DeviceState_index)-1) {
		return fmt.Sprintf("DeviceState(%d)", i)
	}
	return _DeviceState_name[_DeviceState_index[i]:_DeviceState_index[i+1]]
}
