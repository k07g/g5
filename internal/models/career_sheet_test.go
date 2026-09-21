package models

import "testing"

func TestCareerSheet_Normalize(t *testing.T) {
	sheet := &CareerSheet{}
	sheet.Normalize()

	if sheet.WorkExperiences == nil {
		t.Error("WorkExperiences should be normalized to a non-nil empty slice")
	}
	if sheet.Skills == nil {
		t.Error("Skills should be normalized to a non-nil empty slice")
	}
	if sheet.Educations == nil {
		t.Error("Educations should be normalized to a non-nil empty slice")
	}
	if sheet.Certifications == nil {
		t.Error("Certifications should be normalized to a non-nil empty slice")
	}

	t.Run("does not touch already-populated slices", func(t *testing.T) {
		populated := &CareerSheet{
			WorkExperiences: []WorkExperience{{ID: "1"}},
		}
		populated.Normalize()
		if len(populated.WorkExperiences) != 1 {
			t.Errorf("WorkExperiences = %v, want it left untouched", populated.WorkExperiences)
		}
	})
}
