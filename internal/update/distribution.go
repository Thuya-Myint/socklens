package update

// distribution is set by the linker for Microsoft Store builds. It selects
// update ownership; it is a build setting, not an application trust check.
var distribution string

func StoreManaged() bool { return distribution == "microsoft-store" }
