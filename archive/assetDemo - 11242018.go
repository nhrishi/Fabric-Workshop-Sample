package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	pb "github.com/hyperledger/fabric/protos/peer"
)

// AssetChaincode example Asset Chaincode implementation
type AssetChaincode struct {
}

type asset struct {
	ObjectType string `json:"objectType"` //objectType is used to distinguish the various types of objects in state database
	Name       string `json:"name"`    //the fieldtags are needed to keep case from bouncing around
	Quantity   int    `json:"quantity"`
	Owner      string `json:"owner"`
	Active	   string `json:"active"`
}

// ===================================================================================
// Main
// ===================================================================================
func main() {
	err := shim.Start(new(AssetChaincode))
	if err != nil {
		fmt.Printf("Error starting Asset chaincode: %s", err)
	}
}

// Init initializes chaincode
// ===========================
func (t *AssetChaincode) Init(stub shim.ChaincodeStubInterface) pb.Response {
	fmt.Println("Initialisation Successful!!")
	return shim.Success(nil)
}

// Invoke - Our entry point for Invocations
// ========================================
func (t *AssetChaincode) Invoke(stub shim.ChaincodeStubInterface) pb.Response {
	function, args := stub.GetFunctionAndParameters()
	fmt.Println("invoke is running " + function)

	// Handle different functions
	switch function {
	case "issueAsset":
		//create a new asset
		return t.issueAsset(stub, args)
	case "readAsset":
		//read a asset
		return t.readAsset(stub, args)
	case "transferAsset":
		//change owner of a specific asset
		return t.transferAsset(stub, args)
	case "queryAssetsByOwner":
		//find assets for owner X using rich query
		return t.queryAssetsByOwner(stub, args)
	default:
		//error
		fmt.Println("invoke did not find func: " + function)
		return shim.Error("Received unknown function invocation")
	}
}

// ============================================================
// issueAsset - create a new asset, store into chaincode state
// ============================================================
func (t *AssetChaincode) issueAsset(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	var err error

	//  0-name  1-quantity  2-owner  
	// "USD",  "1000000",  "Hrishi"
	if len(args) != 3 {
		return shim.Error("Incorrect number of arguments. Expecting 3")
	}

	// ==== Input sanitation ====
	fmt.Println("- start init asset")
	if len(args[0]) == 0 {
		return shim.Error("1st argument must be a non-empty string")
	}
	if len(args[1]) == 0 {
		return shim.Error("2nd argument must be a non-empty string")
	}
	if len(args[2]) == 0 {
		return shim.Error("3rd argument must be a non-empty string")
	}

	assetName := args[0]
	owner := strings.ToLower(args[2])
	quantity, err := strconv.Atoi(args[1])
	active := "A"
	if err != nil {
		return shim.Error("1st argument must be a numeric string")
	}
	
	// ==== Check if asset already exists ====
	assetAsBytes, err := stub.GetPrivateData("assetCollection", assetName)
	if err != nil {
		return shim.Error("Failed to get asset: " + err.Error())
	} else if assetAsBytes != nil {
		fmt.Println("This asset already exists: " + assetName)
		return shim.Error("This asset already exists: " + assetName)
	}

	// ==== Create asset object and marshal to JSON ====
	objectType := "asset"
	asset := &asset{objectType, assetName, quantity, owner, active}
	assetJSONasBytes, err := json.Marshal(asset)
	if err != nil {
		return shim.Error(err.Error())
	}
	//Alternatively, build the asset json string manually if you don't want to use struct marshalling
	//assetJSONasString := `{"objectType":"asset",  "name": "` + asseyName + `", "quantity": ` + strconv.Itoa(size) + `, "owner": "` + owner + `"}`
	//assetJSONasBytes := []byte(assetJSONasString)

	// === Save asset to state ===
	err = stub.PutPrivateData("assetCollection", assetName, assetJSONasBytes)
	if err != nil {
		return shim.Error(err.Error())
	}

	//  ==== Index the asset to enable owner-based range queries
	//  An 'index' is a normal key/value entry in state.
	//  The key is a composite key, with the elements that you want to range query on listed first.
	//  In our case, the composite key is based on indexName~owner~name.
	//  This will enable very efficient state range queries based on composite keys matching indexName~owner~*
	indexName := "owner~name"
	ownerNameIndexKey, err := stub.CreateCompositeKey(indexName, []string{asset.Owner, asset.Name})
	if err != nil {
		return shim.Error(err.Error())
	}
	//  Save index entry to state. Only the key name is needed, no need to store a duplicate copy of the asset.
	//  Note - passing a 'nil' value will effectively delete the key from state, therefore we pass null character as value
	value := []byte{0x00}
	stub.PutPrivateData("assetCollection", ownerNameIndexKey, value)

	// ==== Asset saved and indexed. Return success ====
	fmt.Println("- end init asset")
	return shim.Success(nil)
}

// ===============================================
// readAsset - read a asset from chaincode state
// ===============================================
func (t *AssetChaincode) readAsset(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	var assetName, jsonResp string
	var err error

	if len(args) != 1 {
		return shim.Error("Incorrect number of arguments. Expecting name of the asset to query")
	}

	assetName = args[0]
	valAsbytes, err := stub.GetPrivateData("assetCollection", assetName) //get the asset from chaincode state
	if err != nil {
		jsonResp = "{\"Error\":\"Failed to get state for " + assetName + "\"}"
		return shim.Error(jsonResp)
	} else if valAsbytes == nil {
		jsonResp = "{\"Error\":\"Asset does not exist: " + assetName + "\"}"
		return shim.Error(jsonResp)
	}

	return shim.Success(valAsbytes)
}

// ===========================================================
// transfer a asset by setting a new owner name on the asset
// ===========================================================
func (t *AssetChaincode) transferAsset(stub shim.ChaincodeStubInterface, args []string) pb.Response {

	//   0       1
	// "name", "newOwner"
	if len(args) < 2 {
		return shim.Error("Incorrect number of arguments. Expecting 2")
	}

	assetName := args[0]
	newOwner := strings.ToLower(args[1])
	fmt.Println("- start transferAsset ", assetName, newOwner)

	assetAsBytes, err := stub.GetPrivateData("assetCollection", assetName)
	if err != nil {
		return shim.Error("Failed to get asset:" + err.Error())
	} else if assetAsBytes == nil {
		return shim.Error("asset does not exist")
	}

	assetToTransfer := asset{}
	err = json.Unmarshal(assetAsBytes, &assetToTransfer) //unmarshal it aka JSON.parse()
	if err != nil {
		return shim.Error(err.Error())
	}
	assetToTransfer.Owner = newOwner //change the owner

	assetJSONasBytes, _ := json.Marshal(assetToTransfer)
	err = stub.PutPrivateData("assetCollection", assetName, assetJSONasBytes) //rewrite the asset
	if err != nil {
		return shim.Error(err.Error())
	}

	fmt.Println("- end transferAsset (success)")
	return shim.Success(nil)
}

// =======Rich queries =========================================================================
// Two examples of rich queries are provided below (parameterized query and ad hoc query).
// Rich queries pass a query string to the state database.
// Rich queries are only supported by state database implementations
//  that support rich query (e.g. CouchDB).
// The query string is in the syntax of the underlying state database.
// With rich queries there is no guarantee that the result set hasn't changed between
//  endorsement time and commit time, aka 'phantom reads'.
// Therefore, rich queries should not be used in update transactions, unless the
// application handles the possibility of result set changes between endorsement and commit time.
// Rich queries can be used for point-in-time queries against a peer.
// ============================================================================================

// ===== Example: Parameterized rich query =================================================
// queryAssetsByOwner queries for assets based on a passed in owner.
// This is an example of a parameterized query where the query logic is baked into the chaincode,
// and accepting a single query parameter (owner).
// Only available on state databases that support rich query (e.g. CouchDB)
// =========================================================================================
func (t *AssetChaincode) queryAssetsByOwner(stub shim.ChaincodeStubInterface, args []string) pb.Response {

	//   0
	// "bob"
	if len(args) < 1 {
		return shim.Error("Incorrect number of arguments. Expecting 1")
	}

	owner := strings.ToLower(args[0])

	queryString := fmt.Sprintf("{\"selector\":{\"objectType\":\"asset\",\"owner\":\"%s\"}}", owner)

	queryResults, err := getQueryResultForQueryString(stub, queryString)
	if err != nil {
		return shim.Error(err.Error())
	}
	return shim.Success(queryResults)
}

// =========================================================================================
// getQueryResultForQueryString executes the passed in query string.
// Result set is built and returned as a byte array containing the JSON results.
// =========================================================================================
func getQueryResultForQueryString(stub shim.ChaincodeStubInterface, queryString string) ([]byte, error) {

	fmt.Printf("- getQueryResultForQueryString queryString:\n%s\n", queryString)

	resultsIterator, err := stub.GetPrivateDataQueryResult("assetCollection", queryString)
	if err != nil {
		return nil, err
	}
	defer resultsIterator.Close()

	// buffer is a JSON array containing QueryRecords
	var buffer bytes.Buffer
	buffer.WriteString("[")

	bArrayMemberAlreadyWritten := false
	for resultsIterator.HasNext() {
		queryResponse, err := resultsIterator.Next()
		if err != nil {
			return nil, err
		}
		// Add a comma before array members, suppress it for the first array member
		if bArrayMemberAlreadyWritten == true {
			buffer.WriteString(",")
		}
		buffer.WriteString("{\"Key\":")
		buffer.WriteString("\"")
		buffer.WriteString(queryResponse.Key)
		buffer.WriteString("\"")

		buffer.WriteString(", \"Record\":")
		// Record is a JSON object, so we write as-is
		buffer.WriteString(string(queryResponse.Value))
		buffer.WriteString("}")
		bArrayMemberAlreadyWritten = true
	}
	buffer.WriteString("]")

	fmt.Printf("- getQueryResultForQueryString queryResult:\n%s\n", buffer.String())

	return buffer.Bytes(), nil
}
