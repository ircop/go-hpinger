package main

import (
	"database/sql"
	"fmt"
	_"github.com/mattn/go-oci8"
	"github.com/spf13/viper"
	_"os"
	"time"
	"sync"
	"os"
	"github.com/go-resty/resty"
	"./ping"
)

var dbLock *sync.Mutex

var aliveParamId	int
var swRootId		int
var sendRest		bool
var restUrl		string
var restPass		string

func main() {
	
	dbLock = &sync.Mutex{}
	
	// Reading config...
	viper.AddConfigPath(".")
	viper.AddConfigPath("/etc/")
	viper.SetConfigName("hpinger")
	err := viper.ReadInConfig()
	
	if err != nil {
		panic(fmt.Errorf("Fatal error: config file: %s \n", err))
	}
	
	dbhost := viper.GetString("dbhost")
	dbuser := viper.GetString("dbuser")
	dbpassword := viper.GetString("dbpassword")
	dbsid := viper.GetString("dbsid")
	swRootId = viper.GetInt("switch_root_id")
	aliveParamId = viper.GetInt("alive_param_id")
	sendRest = viper.GetBool("rest")
	restUrl = viper.GetString("rest_url")
	
	if( dbhost == "" || dbuser == "" || dbpassword == "" || dbsid == "" || swRootId == 0 || aliveParamId == 0 ) {
		panic( fmt.Errorf("Fatal error: missing required config parameters.") )
	}
	
	if( sendRest == true && (restUrl == "" ) ) {
		panic("rest_url or rest_pass is empty!")
	}
	
	
	os.Setenv("NLS_LANG", "AMERICAN_AMERICA.AL32UTF8")
	
	db, err := sql.Open("oci8", fmt.Sprintf("%s/%s@%s", dbuser, dbpassword, dbsid) )
	if err != nil {
		fmt.Println(err)
		return
	}
	defer db.Close()
	
	
	GlobalLoop:
	for {
		err = db.Ping()
		if err != nil {
			fmt.Printf("DB Ping error: %s", err)
			time.Sleep(60 * time.Second)
			
			db,_ = sql.Open("oci8", fmt.Sprintf("%s/%s@%s", dbuser, dbpassword, dbsid) )
			continue GlobalLoop
		}
		
		// switches:
		dbLock.Lock()
		switches, err := db.Query(`
		SELECT O.N_OBJECT_ID, IPADR.VC_VISUAL_CODE AS IP, 
		    V1.VC_VISUAL_VALUE AS ALIVE, 
		    V1.N_OBJ_VALUE_ID AS VALUE_ID,
		    A.VC_VISUAL_CODE AS ADDR,
		    G.VC_NAME AS MODEL
		    FROM SI_V_OBJECTS_SIMPLE O 
			LEFT JOIN SI_V_OBJ_ADDRESSES_SIMPLE_CUR A
			    ON A.N_OBJECT_ID = O.N_OBJECT_ID
			INNER JOIN SR_V_GOODS_SIMPLE G 
			    ON G.N_GOOD_TYPE_ID=1 AND G.N_GOOD_ID=O.N_GOOD_ID 
			INNER JOIN SR_V_GOODS_SIMPLE G2 
			    ON G2.N_GOOD_ID=G.N_PARENT_GOOD_ID 
			INNER JOIN SI_V_OBJECTS_SPEC_SIMPLE OSPEC 
			    ON OSPEC.N_MAIN_OBJECT_ID=O.N_OBJECT_ID AND OSPEC.VC_NAME LIKE 'CPU %' 
			INNER JOIN SI_V_OBJ_ADDRESSES_SIMPLE_CUR IPADR 
			    ON IPADR.N_ADDR_TYPE_ID=SYS_CONTEXT('CONST', 'ADDR_TYPE_IP') AND IPADR.N_OBJECT_ID=OSPEC.N_OBJECT_ID 
			LEFT JOIN SI_V_OBJ_VALUES V1 
			    ON V1.N_OBJECT_ID=O.N_OBJECT_ID AND V1.N_GOOD_VALUE_TYPE_ID=:1
		    WHERE G2.N_PARENT_GOOD_ID=:2
		`, aliveParamId, swRootId)
		
		if( err != nil ) {
			fmt.Println("Error fetching switches\n")
			fmt.Println(err)
			time.Sleep( 60 * time.Second )
			continue
		}
		
		pinger := ping.NewPinger()
		var HostsMap map[string]ping.Host
		HostsMap = make(map[string] ping.Host)
		for switches.Next() {
			var id		string
			var ip		string
			var alive	string
			var addr	string
			var model	string
			var objVal	string
			switches.Scan(&id, &ip, &alive, &objVal, &addr, &model)
			
			var host ping.Host
			host.Ip		= ip
			host.Id		= id
			host.Alive	= false
			host.ObjValue	= objVal
			host.Addr	= addr
			host.Model	= model
			host.OldAliveStr = alive
			if alive == "Y" {
			    host.OldAlive = true
			} else {
			    host.OldAlive = false
			}
			HostsMap[ip] = host
		}
		switches.Close()
		
		dbLock.Unlock()
		
		// max requests at once
		for _, host := range HostsMap {
			pinger.AddHost(host)
		}

		Diffs := pinger.Run()
		pinger.Clear()
		// re-check diffs
		for _, host := range Diffs {
			pinger.AddHost(host)
		}
		Diffs = pinger.Run()
		
		for _, diff := range Diffs {
			fmt.Printf("UPDATING: %s %v -> %v\n", diff.Ip, diff.OldAlive, diff.Alive )
			Update( diff, db )
		}
				
		pinger.Clear()
		fmt.Printf("NEXT iteration...\n")
		
		fmt.Println("Waiting 10 sec...")
		time.Sleep(10 * time.Second)
	}
}

func Update( host ping.Host, db *sql.DB ) {
	dbLock.Lock()
	defer dbLock.Unlock()
	
	if host.OldAliveStr != "N" && host.Alive == false {
		if host.OldAliveStr == "" {
			var tmp string
			_, err := db.Exec("BEGIN SI_OBJECTS_PKG.SI_OBJ_VALUES_PUT( num_N_OBJECT_ID => :1, num_N_GOOD_VALUE_TYPE_ID => :2, ch_C_FL_VALUE => :3, num_N_OBJ_VALUE_ID => :4 ); END;", host.Id, aliveParamId, "N", tmp  )
			if err != nil {
				fmt.Println(err)
				return
			}
		} else if host.OldAliveStr == "Y" {
			_, err := db.Exec("BEGIN SI_OBJECTS_PKG.SI_OBJ_VALUES_PUT( num_N_OBJECT_ID => :id, num_N_GOOD_VALUE_TYPE_ID => :type_id, ch_C_FL_VALUE => :clive, num_N_OBJ_VALUE_ID => :num ); END;", host.Id, aliveParamId, "N", host.ObjValue )
			if err != nil {
				fmt.Println(err)
				return
			}
		}
	}
	
	if host.OldAliveStr != "Y" && host.Alive == true {
		if host.OldAliveStr == "" {
			var tmp string
			_, err := db.Exec("BEGIN SI_OBJECTS_PKG.SI_OBJ_VALUES_PUT( num_N_OBJECT_ID => :1, num_N_GOOD_VALUE_TYPE_ID => :2, ch_C_FL_VALUE => :3, num_N_OBJ_VALUE_ID => :4 ); END;", host.Id, aliveParamId, "Y", tmp  )
			if err != nil {
				fmt.Println(err)
				return
			}
		} else if host.OldAliveStr == "N" {
			_, err := db.Exec("BEGIN SI_OBJECTS_PKG.SI_OBJ_VALUES_PUT( num_N_OBJECT_ID => :id, num_N_GOOD_VALUE_TYPE_ID => :type_id, ch_C_FL_VALUE => :clive, num_N_OBJ_VALUE_ID => :num ); END;", host.Id, aliveParamId, "Y", host.ObjValue )
			if err != nil {
				fmt.Println(err)
				return
			}
		}
	}
	
	if( sendRest == true ) {
		var slive string
		if host.Alive == true {
			slive = "Y"
		} else {
			slive = "N"
		}
		
		_, err := resty.R().
			SetBody(map[string]interface{}{"id":host.Id, "ip":host.Ip, "alive":slive, "addr":host.Addr, "model":host.Model}).
			Post(restUrl)
		if err != nil {
			fmt.Printf("REST Error: %s\n", err)
			return
		}
	}
}
