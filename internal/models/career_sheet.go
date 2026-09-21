package models

// BasicInfo, WorkExperience, SkillItem, Education, Certification and
// CareerSheet mirror the TypeScript types in career-sheet's
// src/types/career-sheet.ts (github.com/k07g/career-sheet) field for
// field, so the JSON stored here round-trips through the frontend without
// any translation layer. The bson tags mirror the json tags so the same
// struct also serializes consistently when embedded in a MongoDB document.

type BasicInfo struct {
	Name      string `json:"name" bson:"name"`
	NameKana  string `json:"nameKana" bson:"nameKana"`
	BirthDate string `json:"birthDate" bson:"birthDate"`
	Email     string `json:"email" bson:"email"`
	Phone     string `json:"phone" bson:"phone"`
	Address   string `json:"address" bson:"address"`
}

type WorkExperience struct {
	ID             string `json:"id" bson:"id"`
	CompanyName    string `json:"companyName" bson:"companyName"`
	EmploymentType string `json:"employmentType" bson:"employmentType"`
	StartDate      string `json:"startDate" bson:"startDate"`
	EndDate        string `json:"endDate" bson:"endDate"`
	IsCurrent      bool   `json:"isCurrent" bson:"isCurrent"`
	Position       string `json:"position" bson:"position"`
	Description    string `json:"description" bson:"description"`
	Technologies   string `json:"technologies" bson:"technologies"`
}

type SkillItem struct {
	ID       string `json:"id" bson:"id"`
	Category string `json:"category" bson:"category"`
	Name     string `json:"name" bson:"name"`
	Level    string `json:"level" bson:"level"`
}

type Education struct {
	ID         string `json:"id" bson:"id"`
	SchoolName string `json:"schoolName" bson:"schoolName"`
	Major      string `json:"major" bson:"major"`
	StartDate  string `json:"startDate" bson:"startDate"`
	EndDate    string `json:"endDate" bson:"endDate"`
}

type Certification struct {
	ID           string `json:"id" bson:"id"`
	Name         string `json:"name" bson:"name"`
	AcquiredDate string `json:"acquiredDate" bson:"acquiredDate"`
}

type CareerSheet struct {
	BasicInfo       BasicInfo        `json:"basicInfo" bson:"basicInfo"`
	Summary         string           `json:"summary" bson:"summary"`
	WorkExperiences []WorkExperience `json:"workExperiences" bson:"workExperiences"`
	Skills          []SkillItem      `json:"skills" bson:"skills"`
	Educations      []Education      `json:"educations" bson:"educations"`
	Certifications  []Certification  `json:"certifications" bson:"certifications"`
	SelfPromotion   string           `json:"selfPromotion" bson:"selfPromotion"`
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
