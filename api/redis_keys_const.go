package api

// PiesAvailableKey is the key representing the set of available pies left to purchase
const PiesAvailableKey = "pies:available"

// PiesJSONKey is the key representing the set of all pies in json format
const PiesJSONKey = "pies:json"

// PiesTotalKey is the key representing the set of all pies
const PiesTotalKey = "pies:total"

// PieKey is the formatted string that represents the key to get a specific pie's
// JSON stringified representation
const PieKey string = "pie:%s"

// HPieKey is the formatted string that represents the key to get a specific pie and it's fields
const HPieKey string = "hpie:%s"

// PieSlicesKey is the formatted string that represents the key to get the number
// of slices for a specific pie
const PieSlicesKey = "pie:%s:slices"

// PiePurchasersKey is the formatted string that represents the number of users
// who has purchased a specific pie
const PiePurchasersKey = "pie:%s:purchasers"

// LabelKey is the formatted string that represents a label. This key points to a
// set of all Pies that are under that label
const LabelKey = "label:%s"

// PurchaseKey is the formatted string that represents the purchases
// by a specific user for a specific Pie.
const PurchaseKey = "pie:%s:user:%s"

// UserAvailableKey is the formatted string that represents the key to the
// number of remaining pies available to the user
const UserAvailableKey = "user:%s:available"

// UserUnavailableKey is the formatted string that represents the key to the
// number of remaining pies that are no longer available to the user due to
// max consumption of slices (3)
const UserUnavailableKey = "user:%s:unavailable"
