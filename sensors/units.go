package sensors

import (
	"strings"
)

// define unit conversion routines
type UnitConverterFunc func(float64) float64
type UnitConverter struct {
	SrcType string
	DstType string
	Convert UnitConverterFunc
}

func KM_H_2_M_S(v float64) float64 {
	return v / 3.6
}

func MI_H_2_M_S(v float64) float64 {
	return v / 2.237
}

func F_2_C(v float64) float64 {
	return (v - 32) * 5 / 9
}

func IN_2_MM(v float64) float64 {
	return v * 25.4
}

func KPA_2_HPA(v float64) float64 {
	return v * 10
}

func PSI_2_HPA(v float64) float64 {
	return v * 68.9475729318
}

func NO_CONV(v float64) float64 {
	return v
}

var convTable = map[string]*UnitConverter{
	"_km_h": {"km_h", "m_s", KM_H_2_M_S},
	"_mi_h": {"mi_h", "m_s", MI_H_2_M_S},
	"_f":    {"f", "c", F_2_C},
	"_in":   {"in", "mm", IN_2_MM},
	"_kpa":  {"kpa", "hpa", KPA_2_HPA},
	"_psi":  {"psi", "hpa", PSI_2_HPA},
}

func GetMeasurementConverter(typedUnit string) (string, *UnitConverter) {
	uTU := strings.ToLower(typedUnit)

	for unit, conv := range convTable {
		if strings.HasSuffix(uTU, unit) {
			newUnit := strings.TrimSuffix(typedUnit, conv.SrcType) + conv.DstType
			return newUnit, conv
		}
	}

	return uTU, nil
}
