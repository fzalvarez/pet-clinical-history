package events

type EventType string

const (
	EventTypeProfileUpdated  EventType = "PROFILE_UPDATED"
	EventTypeWeightRecorded  EventType = "WEIGHT_RECORDED"
	EventTypeNote            EventType = "NOTE"
	EventTypeMedicalVisit    EventType = "MEDICAL_VISIT"
	EventTypeMedicationPresc EventType = "MEDICATION_PRESCRIBED"
	EventTypeVaccine         EventType = "VACCINE"
	EventTypeDeworming       EventType = "DEWORMING"
	EventTypeFleaTreatment   EventType = "FLEA_TREATMENT"
	EventTypeBath            EventType = "BATH"
	EventTypeAttachmentAdded EventType = "ATTACHMENT_ADDED"
)

type ActorType string

const (
	ActorTypeOwnerUser      ActorType = "OWNER_USER"
	ActorTypeDelegateUser   ActorType = "DELEGATE_USER"
	ActorTypeExternalSystem ActorType = "EXTERNAL_SYSTEM"
)

type Source string

const (
	SourceManual      Source = "manual"
	SourceSmartPet    Source = "smartpet"
	SourceIntegration Source = "integration"
)

type Visibility string

const (
	VisibilityPrivate Visibility = "private"
	VisibilityShared  Visibility = "shared_with_delegates"
)

type EventStatus string

const (
	EventStatusActive EventStatus = "active"
	EventStatusVoided EventStatus = "voided"
)
