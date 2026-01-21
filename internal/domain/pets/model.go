package pets

import "time"

// Species define las especies soportadas.
// @Enum dog, cat
type Species string

const (
	SpeciesDog Species = "dog"
	SpeciesCat Species = "cat"
)

// DogBreed define las razas de perro principales.
type DogBreed string

const (
	BreedLabrador        DogBreed = "labrador"
	BreedGoldenRetriever DogBreed = "golden_retriever"
	BreedGermanShepherd  DogBreed = "german_shepherd"
	BreedBulldog         DogBreed = "bulldog"
	BreedPoodle          DogBreed = "poodle"
	BreedChihuahua       DogBreed = "chihuahua"
	BreedBeagle          DogBreed = "beagle"
	BreedDogOther        DogBreed = "other"
)

// CatBreed define las razas de gato principales.
type CatBreed string

const (
	BreedPersian   CatBreed = "persian"
	BreedSiamese   CatBreed = "siamese"
	BreedMaineCoon CatBreed = "maine_coon"
	BreedBengal    CatBreed = "bengal"
	BreedSphynx    CatBreed = "sphynx"
	BreedCommon    CatBreed = "common"
	BreedCatOther  CatBreed = "other"
)

// Sex define el sexo de la mascota.
// @Enum male, female, unknown
type Sex string

const (
	SexMale    Sex = "male"
	SexFemale  Sex = "female"
	SexUnknown Sex = "unknown"
)

// Pet representa el perfil básico de una mascota registrada en el sistema.
type Pet struct {
	ID          string
	OwnerUserID string

	Name    string
	Species Species // dog, cat
	Breed   string  // Según especie (DogBreed o CatBreed)
	Sex     Sex     // male, female, unknown

	BirthDate *time.Time
	Microchip string

	Notes string

	CreatedAt time.Time
	UpdatedAt time.Time
}
