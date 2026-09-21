package models

// BasicInfo, WorkExperience, SkillItem, Education, Certification and
// CareerSheet mirror the TypeScript types in career-sheet's
// src/types/career-sheet.ts (github.com/k07g/career-sheet) field for
// field, so the JSON stored here round-trips through the frontend without
// any translation layer.

type BasicInfo struct {
	Name      string `json:"name"`
	NameKana  string `json:"nameKana"`
	BirthDate string `json:"birthDate"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	Address   string `json:"address"`
}

type WorkExperience struct {
	ID             string `json:"id"`
	CompanyName    string `json:"companyName"`
	EmploymentType string `json:"employmentType"`
	StartDate      string `json:"startDate"`
	EndDate        string `json:"endDate"`
	IsCurrent      bool   `json:"isCurrent"`
	Position       string `json:"position"`
	Description    string `json:"description"`
	Technologies   string `json:"technologies"`
}

type SkillItem struct {
	ID       string `json:"id"`
	Category string `json:"category"`
	Name     string `json:"name"`
	Level    string `json:"level"`
}

type Education struct {
	ID         string `json:"id"`
	SchoolName string `json:"schoolName"`
	Major      string `json:"major"`
	StartDate  string `json:"startDate"`
	EndDate    string `json:"endDate"`
}

type Certification struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	AcquiredDate string `json:"acquiredDate"`
}

type CareerSheet struct {
	BasicInfo       BasicInfo        `json:"basicInfo"`
	Summary         string           `json:"summary"`
	WorkExperiences []WorkExperience `json:"workExperiences"`
	Skills          []SkillItem      `json:"skills"`
	Educations      []Education      `json:"educations"`
	Certifications  []Certification  `json:"certifications"`
	SelfPromotion   string           `json:"selfPromotion"`
}

// Normalize replaces nil slices with empty ones so a career sheet always
// serializes its list fields as `[]` rather than `null`. The frontend calls
// .map() on these fields unconditionally, so a `null` would break rendering
// for a sheet whose list fields were never set.
func (c *CareerSheet) Normalize() {
	if c.WorkExperiences == nil {
		c.WorkExperiences = []WorkExperience{}
	}
	if c.Skills == nil {
		c.Skills = []SkillItem{}
	}
	if c.Educations == nil {
		c.Educations = []Education{}
	}
	if c.Certifications == nil {
		c.Certifications = []Certification{}
	}
}
