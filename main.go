package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"
)

type personID [2]int

type personStates struct {
	Healthy        string
	Susceptible    string
	Infected       string
	Ill            string
	UnderTreatment string
	ICU            string
	Recovered      string
	Dead           string
}

func newpersonStates() *personStates {
	return &personStates{
		Healthy:        "healthy",
		Susceptible:    "susceptible",    // = exposed
		Infected:       "infected",       // = asymptomatic
		Ill:            "ill",            // = symptomatic
		UnderTreatment: "underTreatment", // = hospitalization
		ICU:            "icu",            // = ventilation / ICU
		Recovered:      "recovered",      // = positive outcome
		Dead:           "dead",           // = negative outcome
	}
}

// FIXME: see type severityLevelDistribution map[string]int
type sicknessSeverityLevels struct {
	Low      int // 30% NS (asymptomatic) / Recovery
	Mild     int // 56% 5D NS / 5-6D Symptomatic / Recovery
	Severe   int // 10% 5D NS / 5-6D S / 7-8D Hospitalization / Recovery
	Critical int // 4% 5D NS / 5-6D S / 5-6D H / 8-9D ICU / Death
}

func newsicknessSeverityLevels() *sicknessSeverityLevels {
	return &sicknessSeverityLevels{
		Low:      100,
		Mild:     70,
		Severe:   14,
		Critical: 4,
	}
}

type severityLevelDistribution map[string]int
type contactsPerDayModifiers map[string]float64
type mortalityAmongAgeGroups map[int]float64

type mainParametersStruct struct {
	TotalPopulation         int
	InfectionRate           int
	TransitionRate          int
	MortalityRate           int
	MaximumContactsPerDay   int
	MaximumTravelRange      int
	GrayPeriod              int
	SelfRecoveryRate        int
	DaysBeforeSelfRecovery  int
	HealthcareCapacity      int
	SelfIsolationRate       int
	SelfIsolationStrictness int
	TotalQuarantineTreshold int
	severityLevelDistribution
	contactsPerDayModifiers
	mortalityAmongAgeGroups
}

func readJSON(fn string, v interface{}) {
	file, _ := os.Open(fn)
	defer file.Close()
	decoder := json.NewDecoder(file)
	err := decoder.Decode(v)
	if err != nil {
		log.Println("error:", err)
	}
}

var mainParameters mainParametersStruct

var personState = newpersonStates()
var sicknessSeverity = newsicknessSeverityLevels()

type citizen struct {
	state            string
	daysInState      int
	selfIsolated     bool //self-isolation restricts daily contacts with a SelfIsolationStrictness probability
	hospitality      int  //the more hospitality the more total nember of contacts per day to allowed maximum of MaximumContactsPerDay
	sicknessSeverity int  //defines a probability to recover without medical treatment
	age              int  //current age
	personID              //person's Digital Passport :)
}

const populationSpaceDimension = 5

type populationType [populationSpaceDimension][populationSpaceDimension]citizen

type globalStatsStruct struct {
	totalInfected     int
	totalRecovered    int
	totalIll          int
	totalDead         int
	totalIntact       int
	totalSelfIsolated int
	currentMortality  int
	daysCount         int
	totalQuarantine   bool
}

var globalStats globalStatsStruct

func (globalStats globalStatsStruct) String() string {
	return fmt.Sprintf("Day: %v\nDead: %v\nIll: %v\nInfected: %v\nSelf-isolated: %v\nRecovered: %v\nIntact: %v\nCurrent mortality: %v",
		// return fmt.Sprintf("%v,%v,%v,%v,%v",
		globalStats.daysCount,
		globalStats.totalDead,
		globalStats.totalIll,
		globalStats.totalInfected,
		globalStats.totalSelfIsolated,
		globalStats.totalRecovered,
		globalStats.totalIntact,
		globalStats.currentMortality)
}

const enableDebugMessages = false

func (p *populationType) getContacted(referencePerson citizen, radius, maximumContacts int) []personID {

	var neighboursArray []personID

	var allNeighbours []personID
	for hOffset := -radius; hOffset <= radius; hOffset++ {
		for vOffset := -radius; vOffset <= radius; vOffset++ {
			if (hOffset == 0) && (vOffset == 0) {
				continue
			}
			k := referencePerson.personID[0] + hOffset
			m := referencePerson.personID[1] + vOffset

			switch {
			case k < 0:
				k = populationSpaceDimension + k
			case k >= populationSpaceDimension:
				k = k - populationSpaceDimension
			}

			switch {
			case m < 0:
				m = populationSpaceDimension + m
			case m >= populationSpaceDimension:
				m = m - populationSpaceDimension
			}

			allNeighbours = append(allNeighbours, personID{k, m})
		}
	}

	//pick "maximum" number of points as a result
	candidatesToBePicked := maximumContacts

	s1 := rand.NewSource(time.Now().UnixNano())
	r1 := rand.New(s1)

	for _, candidate := range allNeighbours {

		if r1.Intn(100) <= referencePerson.hospitality {
			neighboursArray = append(neighboursArray, candidate) //personID{candidate[0], candidate[1]})
			candidatesToBePicked--
		}
		if candidatesToBePicked == 0 {
			break
		}

	}

	return neighboursArray
}

func checkError(message string, err error) {
	if err != nil {
		log.Fatal(message, err)
	}
}

func removeSick(pArray []personID, index int) []personID {
	if index <= len(pArray)-1 {
		pArray[index] = pArray[len(pArray)-1] // Copy last element to index i.
		pArray[len(pArray)-1] = personID{}    // Erase last element (write zero value).
		pArray = pArray[:len(pArray)-1]
	}

	return pArray
}

func main() {
	mainParameters = mainParametersStruct{}
	//FIXME: resolve potential descriptive parameter doubling
	mainParameters.severityLevelDistribution = make(severityLevelDistribution)
	mainParameters.severityLevelDistribution["Critical"] = 4
	mainParameters.severityLevelDistribution["Severe"] = 10
	mainParameters.severityLevelDistribution["Mild"] = 56
	mainParameters.severityLevelDistribution["Low"] = 30

	mainParameters.contactsPerDayModifiers = make(contactsPerDayModifiers)
	mainParameters.contactsPerDayModifiers[personState.Healthy] = 1.0
	mainParameters.contactsPerDayModifiers[personState.Recovered] = 1.0
	mainParameters.contactsPerDayModifiers[personState.Susceptible] = 0.5
	mainParameters.contactsPerDayModifiers[personState.Ill] = 0.5
	mainParameters.contactsPerDayModifiers[personState.Infected] = 0.5
	mainParameters.contactsPerDayModifiers[personState.UnderTreatment] = 0.06
	mainParameters.contactsPerDayModifiers[personState.ICU] = 0.01
	mainParameters.contactsPerDayModifiers[personState.Dead] = 0.0

	mainParameters.mortalityAmongAgeGroups = make(mortalityAmongAgeGroups)
	mainParameters.mortalityAmongAgeGroups[9] = 0.0
	mainParameters.mortalityAmongAgeGroups[39] = 0.2
	mainParameters.mortalityAmongAgeGroups[49] = 0.4
	mainParameters.mortalityAmongAgeGroups[59] = 1.3
	mainParameters.mortalityAmongAgeGroups[69] = 3.6
	mainParameters.mortalityAmongAgeGroups[79] = 8.0
	mainParameters.mortalityAmongAgeGroups[99] = 14.8

	readJSON("config.json", &mainParameters)

	//FIXME: find out how to set a range dynamically
	// const count = int(math.Floor(math.Sqrt(float64(mainParameters.TotalPopulation))))

	file, err := os.Create("result.csv")
	checkError("Cannot create file", err)
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	//initialize
	s1 := rand.NewSource(time.Now().UnixNano())
	r1 := rand.New(s1)

	var population populationType
	globalStats = globalStatsStruct{}

	for i := 0; i < populationSpaceDimension; i++ {
		for j := 0; j < populationSpaceDimension; j++ {
			population[i][j] = citizen{state: personState.Healthy, personID: personID{i, j}, hospitality: r1.Intn(100) + 50, sicknessSeverity: r1.Intn(4)}
		}
	}

	mainParameters.TotalPopulation = populationSpaceDimension * populationSpaceDimension

	globalStats.totalIntact = mainParameters.TotalPopulation

	var pArrayOfSick []personID

	// a random person gets ill
	iVeryFirstInfected := r1.Intn(populationSpaceDimension)
	jVeryFirstInfected := r1.Intn(populationSpaceDimension)

	population[iVeryFirstInfected][jVeryFirstInfected].state = personState.Ill
	population[iVeryFirstInfected][jVeryFirstInfected].daysInState = 1

	pArrayOfSick = append(pArrayOfSick, population[iVeryFirstInfected][jVeryFirstInfected].personID)

	globalStats.totalIll++
	globalStats.totalIntact--

	writer.Write([]string{"Day", "Dead", "Ill", "Infected", "Recovered", "Healthcare capacity", "Current mortality rate", "Self-isolated"})
	line := []string{
		fmt.Sprintf("%v", globalStats.daysCount),
		fmt.Sprintf("%v", globalStats.totalDead),
		fmt.Sprintf("%v", globalStats.totalIll),
		fmt.Sprintf("%v", globalStats.totalInfected),
		fmt.Sprintf("%v", globalStats.totalRecovered),
		fmt.Sprintf("%v", mainParameters.HealthcareCapacity),
		fmt.Sprintf("%v", globalStats.currentMortality),
		fmt.Sprintf("%v", globalStats.totalSelfIsolated),
	}
	writer.Write(line)

	var totalQuarantineAppliedOnPreviousDay = false

	// step over
	for globalStats.totalInfected+globalStats.totalIll > 0 {

		globalStats.daysCount++

		if globalStats.totalInfected >= mainParameters.HealthcareCapacity {
			globalStats.currentMortality = 2 * mainParameters.MortalityRate
		} else {
			globalStats.currentMortality = mainParameters.MortalityRate
		}

		if enableDebugMessages {
			fmt.Printf("%v\n", globalStats)
		}

		s1 := rand.NewSource(time.Now().UnixNano())
		r1 := rand.New(s1)

		for index, element := range pArrayOfSick {
			//1. take a person
			person := &population[element[0]][element[1]]
			person.daysInState++

			if enableDebugMessages {
				fmt.Printf("Person [%v] already %v days in state %v\n", person.personID, person.daysInState, person.state)
			}

			switch person.state {
			// if a person is either recovered or dead, do nothing
			case personState.Recovered:
				//do nothing
				if enableDebugMessages {
					fmt.Printf("Person [%v] already recovered. Skipping\n", person.personID)
				}
			case personState.Dead:
				//do nothing
				if enableDebugMessages {
					fmt.Printf("Person [%v] already dead. Skipping\n", person.personID)
				}
			default:
				switch {
				case globalStats.totalQuarantine:
					//if total strict quarantine applied: no contacts allowed
					break
				case person.selfIsolated && (r1.Intn(100) <= mainParameters.SelfIsolationStrictness):
					//if a person self-isolated, it have no contacts
					break
				}
				// no quarantine, no self-quarantine:
				//2. get neighbours
				neighboursArray := population.getContacted(*person, mainParameters.MaximumTravelRange, mainParameters.MaximumContactsPerDay)
				for _, contactElement := range neighboursArray {
					contact := &population[contactElement[0]][contactElement[1]]
					switch contact.state {
					//3. calculate a chance to infect each of them
					//3.1 leave recovered and dead intact
					case personState.Recovered:
						// do nothing
					case personState.Dead:
						//do nothing
					default:
						//person.ill or person.susceptible and contact.healthy
						switch {
						case ((person.state == personState.Ill) || (person.state == personState.Susceptible)) && (contact.state == personState.Healthy):
							if r1.Intn(100) <= mainParameters.TransitionRate {
								contact.state = personState.Susceptible
								contact.daysInState = 1
								globalStats.totalInfected++
								globalStats.totalIntact--

								pArrayOfSick = append(pArrayOfSick, contact.personID)

								if enableDebugMessages {
									fmt.Println("Contacted person", contact.personID, " gets infected")
								}

							}
						//vise versa: contact.ill or contact.Susceptible and person.healthy
						case ((contact.state == personState.Ill) || (contact.state == personState.Susceptible)) && (person.state == personState.Healthy):
							if r1.Intn(100) <= mainParameters.TransitionRate {
								person.state = personState.Susceptible
								person.daysInState = 1
								globalStats.totalInfected++
								globalStats.totalIntact--

								pArrayOfSick = append(pArrayOfSick, person.personID)

								if enableDebugMessages {
									fmt.Println("Person [", person.personID, "] gets infected after contact")
								}
							}
						default:
							// do nothing
						}
					}

				}
				//3.2 if a person is ill or infected
				switch {
				// get a chance to die
				case (person.state == personState.Ill) && (r1.Intn(100) <= globalStats.currentMortality):
					person.state = personState.Dead
					globalStats.totalDead++
					globalStats.totalIll--

					pArrayOfSick = removeSick(pArrayOfSick, index)

					if enableDebugMessages {
						fmt.Printf("Person [%v] dies after %v days\n", person.personID, person.daysInState)
					}
				//get a chance to get ill
				case (person.state == personState.Susceptible) && (person.daysInState >= mainParameters.GrayPeriod):
					if r1.Intn(100) <= mainParameters.InfectionRate {
						if enableDebugMessages {
							fmt.Printf("Person [%v] gets ill after %v days\n", person.personID, person.daysInState)
						}

						person.state = personState.Ill
						person.daysInState = 1

						// self-isolate
						if r1.Intn(100) <= mainParameters.SelfIsolationRate {
							person.selfIsolated = true
							globalStats.totalSelfIsolated++
						}

						globalStats.totalIll++
						globalStats.totalInfected--

					}
				//get a chance to recover
				case (person.state == personState.Ill) && (person.daysInState >= mainParameters.DaysBeforeSelfRecovery):
					if r1.Intn(100) <= mainParameters.SelfRecoveryRate/2 {
						if enableDebugMessages {
							fmt.Printf("Person [%v] recovers after %v days of illness\n", person.personID, person.daysInState)
						}

						person.state = personState.Recovered
						person.daysInState = 1

						pArrayOfSick = removeSick(pArrayOfSick, index)
						globalStats.totalIll--
						globalStats.totalRecovered++
					}
				//get a chance to get sick
				case (person.state == personState.Susceptible) && (person.daysInState >= mainParameters.GrayPeriod):
					if r1.Intn(100) <= mainParameters.InfectionRate {
						if enableDebugMessages {
							fmt.Printf("Person [%v] gets ill after %v days of being infected\n", person.personID, person.daysInState)
						}

						person.state = personState.Ill
						person.daysInState = 1

						globalStats.totalInfected--
						globalStats.totalIll++
					}

				case (person.state == personState.Susceptible) && (person.daysInState >= mainParameters.DaysBeforeSelfRecovery):
					if r1.Intn(100) <= mainParameters.SelfRecoveryRate {
						if enableDebugMessages {
							fmt.Printf("Person [%v] recovers after %v days of being infected\n", person.personID, person.daysInState)
						}

						person.state = personState.Recovered
						person.daysInState = 1

						pArrayOfSick = removeSick(pArrayOfSick, index)
						globalStats.totalRecovered++
						globalStats.totalInfected--
					}
				//stay at current condition one more day
				default:
					// do nothing
				}

			}
		}

		globalStats.totalQuarantine = (((globalStats.totalIll + globalStats.totalDead) * 100 / mainParameters.TotalPopulation) > mainParameters.TotalQuarantineTreshold)

		switch {
		case globalStats.totalQuarantine && !totalQuarantineAppliedOnPreviousDay:
			totalQuarantineAppliedOnPreviousDay = true
			fmt.Printf("Day %v. Total quarantine applied\n", globalStats.daysCount)
		case !globalStats.totalQuarantine && totalQuarantineAppliedOnPreviousDay:
			totalQuarantineAppliedOnPreviousDay = false
			fmt.Printf("Day %v. Total quarantine dismissed\n", globalStats.daysCount)
		}

		line := []string{
			fmt.Sprintf("%v", globalStats.daysCount),
			fmt.Sprintf("%v", globalStats.totalDead),
			fmt.Sprintf("%v", globalStats.totalIll),
			fmt.Sprintf("%v", globalStats.totalInfected),
			fmt.Sprintf("%v", globalStats.totalRecovered),
			fmt.Sprintf("%v", mainParameters.HealthcareCapacity),
			fmt.Sprintf("%v", globalStats.currentMortality),
			fmt.Sprintf("%v", globalStats.totalSelfIsolated),
		}
		writer.Write(line)

	}
	fmt.Println(globalStats)
	fmt.Println("End of sumilation")
}
