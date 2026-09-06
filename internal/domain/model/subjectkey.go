package model

// SubjectKeyField is the context field the static and percentage strategies key
// off: a map lookup for one, a hash bucket for the other.
//
// The name is fixed rather than configurable per segment. It is the contract
// between the caller and those two strategies, and validation refuses a static
// or percentage segment whose layer does not declare it — so a layer that
// dropped the field is a load error rather than a silent misbehaviour. Every
// other strategy never reads it, and a layer built from them need not declare
// it, which is the whole reason it stopped being a required request field.
//
// It lives here rather than beside the evaluator because validation needs it
// too, and validation cannot import the engine — the dependency runs the other
// way.
const SubjectKeyField = "subjectKey"
