package main

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/hyperledger/fabric/core/chaincode/shim"
)

var logger = shim.NewLogger("CLDChaincode")

//==============================================================================================================================
//	 Participant types - Each participant type is mapped to an integer which we use to compare to the value stored in a
//						 user's eCert
//==============================================================================================================================
//CURRENT WORKAROUND USES ROLES CHANGE WHEN OWN USERS CAN BE CREATED SO THAT IT READ 1, 2, 3, 4, 5
const AUTHORITY = "regulator"
const MANUFACTURER = "manufacturer"
const PRIVATE_ENTITY = "private"
const LEASE_COMPANY = "lease_company"
const SCRAP_MERCHANT = "scrap_merchant"

//==============================================================================================================================
//	 Status types - Asset lifecycle is broken down into 5 statuses, this is part of the business logic to determine what can
//					be done to the vehicle at points in it's lifecycle
//==============================================================================================================================
const STATE_TEMPLATE = 0
const STATE_MANUFACTURE = 1
const STATE_PRIVATE_OWNERSHIP = 2
const STATE_LEASED_OUT = 3
const STATE_BEING_SCRAPPED = 4

//==============================================================================================================================
//	 Structure Definitions
//==============================================================================================================================
//	Chaincode - A blank struct for use with Shim (A HyperLedger included go file used for get/put state
//				and other HyperLedger functions)
//==============================================================================================================================
type SimpleChaincode struct {
}

//==============================================================================================================================
//	Vehicle - Defines the structure for a car object. JSON on right tells it what JSON fields to map to
//			  that element when reading a JSON object into the struct e.g. JSON make -> Struct Make.
//==============================================================================================================================

type Bond struct {
	ID              string `json:"id"`
	RealEstateID    string `json:"id"`                // blueprint_number.readestate_number ex: 1232.21
	OwnerNationalID string `json:"owner_national_id"` // national_id
	Status          string `json:"status"`            // flat, built
	Area            string `json:"area"`              // example:
	Coordinates     struct {
		Long string `json:"long"`
		Lat  string `json:"lat"`
	} `json:"coordinates"`
	Borders struct {
		North string `json:"north"`
		South string `json:"south"`
		East  string `json:"east"`
		West  string `json:"west"`
	} `json:"borders"`
}

//==============================================================================================================================
//	V5C Holder - Defines the structure that holds all the v5cIDs for vehicles that have been created.
//				Used as an index when querying all vehicles.
//==============================================================================================================================

type Bond_Holder struct {
	BondIDs []string `json:"bond_ids"`
}

//=============================================================================================================
//	User_and_eCert - Struct for storing the JSON of a user and their ecert
//==============================================================================================================================

type User_and_eCert struct {
	Identity string `json:"identity"`
	ECert    string `json:"ecert"`
}

//==============================================================================================================================
//	Init Function - Called when the user deploys the chaincode
//==============================================================================================================================
func (t *SimpleChaincode) Init(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {

	//Args
	//				0
	//			peer_address

	var bondIDs Bond_Holder

	bytes, err := json.Marshal(bondIDs)

	if err != nil {
		return nil, errors.New("Error creating RealEstateBond_Holder record")
	}

	err = stub.PutState("bondIDs", bytes)
	// TODO: modify the cert for users.
	/*for i := 0; i < len(args); i = i + 2 {
		t.add_ecert(stub, args[i], args[i+1])
	}*/

	return nil, nil
}

//==============================================================================================================================
//	 General Functions
//==============================================================================================================================
//	 get_ecert - Takes the name passed and calls out to the REST API for HyperLedger to retrieve the ecert
//				 for that user. Returns the ecert as retrived including html encoding.
//==============================================================================================================================
func (t *SimpleChaincode) get_ecert(stub shim.ChaincodeStubInterface, name string) ([]byte, error) {

	ecert, err := stub.GetState(name)

	if err != nil {
		return nil, errors.New("Couldn't retrieve ecert for user " + name)
	}

	return ecert, nil
}

//==============================================================================================================================
//	 add_ecert - Adds a new ecert and user pair to the table of ecerts
//==============================================================================================================================

func (t *SimpleChaincode) add_ecert(stub shim.ChaincodeStubInterface, name string, ecert string) ([]byte, error) {

	err := stub.PutState(name, []byte(ecert))

	if err == nil {
		return nil, errors.New("Error storing eCert for user " + name + " identity: " + ecert)
	}

	return nil, nil

}

//==============================================================================================================================
//	 check_affiliation - Takes an ecert as a string, decodes it to remove html encoding then parses it and checks the
// 				  		certificates common name. The affiliation is stored as part of the common name.
//==============================================================================================================================

//==============================================================================================================================
//	 get_caller_data - Calls the get_ecert and check_role functions and returns the ecert and role for the
//					 name passed.
//==============================================================================================================================
//==============================================================================================================================
//	 retrieve_v5c - Gets the state of the data at v5cID in the ledger then converts it from the stored
//					JSON into the Vehicle struct for use in the contract. Returns the Vehcile struct.
//					Returns empty v if it errors.
//==============================================================================================================================

func (t *SimpleChaincode) retrieve_bond(stub shim.ChaincodeStubInterface, ReadEstateID string) (Bond, error) {
	var b Bond
	bytes, err := stub.GetState(ReadEstateID)

	if err != nil {
		fmt.Printf("RETRIEVE_V5C: Failed to invoke vehicle_code: %s", err)
		return b, errors.New("RETRIEVE_V5C: Error retrieving bond with realEstateID = " + ReadEstateID)
	}
	err = json.Unmarshal(bytes, &b)

	if err != nil {
		fmt.Printf("RETRIEVE_BOND: Corrupt vehicle record "+string(bytes)+": %s", err)
		return b, errors.New("RETRIEVE_BOND: Corrupt bond record" + string(bytes))
	}

	return b, nil
}

//==============================================================================================================================
// save_changes - Writes to the ledger the Vehicle struct passed in a JSON format. Uses the shim file's
//				  method 'PutState'.
//==============================================================================================================================
func (t *SimpleChaincode) save_changes(stub shim.ChaincodeStubInterface, b Bond) (bool, error) {

	bytes, err := json.Marshal(b)

	if err != nil {
		fmt.Printf("SAVE_CHANGES: Error converting bond record: %s", err)
		return false, errors.New("Error converting bond record")
	}

	err = stub.PutState(b.RealEstateID, bytes)

	if err != nil {
		fmt.Printf("SAVE_CHANGES: Error storing bond record: %s", err)
		return false, errors.New("Error storing bond record")
	}

	return true, nil
}

//==============================================================================================================================
//	 Router Functions
//==============================================================================================================================
//	Invoke - Called on chaincode invoke. Takes a function name passed and calls that function. Converts some
//		  initial arguments passed to other things for use in the called function e.g. name -> ecert
//==============================================================================================================================
func (t *SimpleChaincode) Invoke(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {

	var b []byte
	if function == "create_bond" {
		return t.create_bond(stub, args)
	} else if function == "ping" {
		return t.ping(stub)
	} else if function == "tranfer_bond" { // If the function is not a create then there must be a car so we need to retrieve the car.
		bond, err := t.retrieve_bond(stub, args[0])
		if err != nil {
			return nil, errors.New("cannot find bond by given realestateID")
		}
		b, err = t.transfer_ownership(stub, bond, args[1])

		if err != nil {
			fmt.Printf("INVOKE: Error retrieving v5c: %s", err)
			return nil, errors.New("Error retrieving v5c")
		}
	} else {
		return nil, errors.New("Function of the name " + function + " doesn't exist.")
	}
	return b, nil
}

//=================================================================================================================================
//	Query - Called on chaincode query. Takes a function name passed and calls that function. Passes the
//  		initial arguments passed are passed on to the called function.
//=================================================================================================================================
func (t *SimpleChaincode) Query(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {

	if function == "get_bond_details" {
		if len(args) != 1 {
			fmt.Printf("Incorrect number of arguments passed")
			return nil, errors.New("QUERY: Incorrect number of arguments passed")
		}
		b, err := t.retrieve_bond(stub, args[0])
		if err != nil {
			fmt.Printf("QUERY: Error retrieving v5c: %s", err)
			return nil, errors.New("QUERY: Error retrieving v5c " + err.Error())
		}
		return t.get_bond_details(stub, b)
	} else if function == "check_unique_real_estate_id" {
		return t.check_unique_read_estate_id(stub, args[0])
	} else if function == "get_bonds" {
		return t.get_bonds(stub)
	} else if function == "get_ecert" {
		return t.get_ecert(stub, args[0])
	} else if function == "ping" {
		return t.ping(stub)
	}

	return nil, errors.New("Received unknown function invocation " + function)

}

//=================================================================================================================================
//	 Ping Function
//=================================================================================================================================
//	 Pings the peer to keep the connection alive
//=================================================================================================================================
func (t *SimpleChaincode) ping(stub shim.ChaincodeStubInterface) ([]byte, error) {
	return []byte("Hello, world!"), nil
}

//=================================================================================================================================
//	 Create Function
//=================================================================================================================================
//	 Create Vehicle - Creates the initial JSON for the vehcile and then saves it to the ledger.
//=================================================================================================================================
func (t *SimpleChaincode) create_bond(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {

	fmt.Println("inside create_bond", args)

	var b Bond

	b.ID = args[0]
	b.RealEstateID = args[1]
	b.OwnerNationalID = args[2]
	b.Status = args[3]
	b.Area = args[4]
	b.Coordinates.Long = args[5]
	b.Coordinates.Lat = args[6]
	b.Borders.North = args[7]
	b.Borders.South = args[8]
	b.Borders.East = args[9]
	b.Borders.West = args[10]

	record, err := stub.GetState(b.RealEstateID) // If not an error then a record exists so cant create a new car with this V5cID as it must be unique

	if record != nil {
		return nil, errors.New("Bond already exists")
	}

	_, err = t.save_changes(stub, b)

	if err != nil {
		fmt.Printf("CREATE_BOND: Error saving changes: %s", err)
		return nil, errors.New("Error saving changes")
	}

	bytes, err := stub.GetState("bondIDs")

	if err != nil {
		return nil, errors.New("Unable to get bondIDs")
	}

	var bondIDs Bond_Holder

	err = json.Unmarshal(bytes, &bondIDs)

	if err != nil {
		return nil, errors.New("Corrupt Bond_Holder record")
	}

	bondIDs.BondIDs = append(bondIDs.BondIDs, b.RealEstateID)

	bytes, err = json.Marshal(bondIDs)

	if err != nil {
		fmt.Print("Error creating V5C_Holder record")
	}

	err = stub.PutState("bondIDs", bytes)

	if err != nil {
		return nil, errors.New("Unable to put the state")
	}

	return nil, nil

}

//=================================================================================================================================
//	 Transfer Functions
//=================================================================================================================================
//	 authority_to_manufacturer
//=================================================================================================================================
func (t *SimpleChaincode) transfer_ownership(stub shim.ChaincodeStubInterface, b Bond, recipient_national_id string) ([]byte, error) {

	b.OwnerNationalID = recipient_national_id // then make the owner the new owner

	_, err := t.save_changes(stub, b) // Write new state

	if err != nil {
		fmt.Printf("AUTHORITY_TO_MANUFACTURER: Error saving changes: %s", err)
		return nil, errors.New("Error saving changes")
	}

	return nil, nil // We are Done

}

//=================================================================================================================================
func (t *SimpleChaincode) get_bond_details(stub shim.ChaincodeStubInterface, b Bond) ([]byte, error) {

	bytes, err := json.Marshal(b)

	if err != nil {
		return nil, errors.New("GET_VEHICLE_DETAILS: Invalid vehicle object")
	}
	return bytes, nil
}

//=================================================================================================================================
//	 get_vehicles
//=================================================================================================================================

func (t *SimpleChaincode) get_bonds(stub shim.ChaincodeStubInterface) ([]byte, error) {
	bytes, err := stub.GetState("bondIDs")

	if err != nil {
		return nil, errors.New("Unable to get bondIDs")
	}

	var bondIDs Bond_Holder

	err = json.Unmarshal(bytes, &bondIDs)

	if err != nil {
		return nil, errors.New("Corrupt Bond_Holder")
	}

	result := "["

	var temp []byte
	var b Bond

	for _, v5c := range bondIDs.BondIDs {

		b, err = t.retrieve_bond(stub, v5c)

		if err != nil {
			return nil, errors.New("Failed to retrieve bondIDs")
		}

		temp, err = t.get_bond_details(stub, b)

		if err == nil {
			result += string(temp) + ","
		}
	}

	if len(result) == 1 {
		result = "[]"
	} else {
		result = result[:len(result)-1] + "]"
	}

	return []byte(result), nil
}

//=================================================================================================================================
//	 check_unique_v5c
//=================================================================================================================================
func (t *SimpleChaincode) check_unique_read_estate_id(stub shim.ChaincodeStubInterface, readEstateID string) ([]byte, error) {
	_, err := t.retrieve_bond(stub, readEstateID)
	if err == nil {
		return []byte("false"), errors.New("ReadEstateID is not unique")
	} else {
		return []byte("true"), nil
	}
}

//=================================================================================================================================
//	 Main - main - Starts up the chaincode
//=================================================================================================================================
func main() {

	err := shim.Start(new(SimpleChaincode))

	if err != nil {
		fmt.Printf("Error starting Chaincode: %s", err)
	}
}
