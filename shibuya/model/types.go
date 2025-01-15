package model

type ShibuyaObject interface {
	*Project | *Collection | *Plan
}
