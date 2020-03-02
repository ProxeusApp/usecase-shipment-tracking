from datetime import datetime
import RPi.GPIO as GPIO
import SimpleMFRC522
import iota
from iota import Address
from iota import Transaction
from iota import TryteString
from iota.crypto.addresses import AddressGenerator
import json
import ConfigParser
import sys
import os
from prettytable import PrettyTable
#import threading
#from gpiozero import LED
from time import sleep
from flask import Flask, request
from flask_json import FlaskJSON, JsonError, json_response, as_json, jsonify
import requests
import cPickle as pickle

#initialize FLASK
app = Flask(__name__)
FlaskJSON(app)

GPIO.setwarnings(False)

#Read ini-file
config = ConfigParser.RawConfigParser()
config.read('sytrax.ini')

#initialize RFID-reader
reader = SimpleMFRC522.SimpleMFRC522()

api = iota.Iota(config.get('MainProd','iotaAPI'))

owner = config.get('MainProd','owner')
terminal = config.get('MainProd','terminal')
IOTAAddress = config.get('MainProd','addressDefault')
seed = config.get('MainProd','seed')
addressIndex = int(config.get('MainProd','addressIndex'))
interface = config.get('MainProd','interface')

#get webhook config
baseHookProtocol = config.get('MainProd','baseHookProtocol')
baseHookHost = config.get('MainProd','baseHookHost')
baseHookURL = config.get('MainProd','baseHookURL')
baseHookCommand = config.get('MainProd','baseHookCommand')
documentIDtrack = config.get('MainProd','documentIDtrack')
documentIDnew = config.get('MainProd','documentIDnew')
documentIDsign = config.get('MainProd','documentIDsign')
callHookTrack = config.get('MainProd','callHookTrack')
callHookNew = config.get('MainProd','callHookNew')
callHookSign = config.get('MainProd','callHookSign')


def sendProxeus(message, documentID):
	hookAddress = baseHookProtocol + baseHookHost + baseHookURL + documentID + "?" + baseHookCommand
	print ("Calling "+hookAddress+" with message\n"+message)
	request = requests.post(hookAddress, json=message)
	print request.text

def generateAddress():
	global addressIndex
	print("Generating a new address")
	generator = AddressGenerator(seed)
	addressIndex += 1
	config.set('MainProd','addressIndex',addressIndex)
	with open('sytrax.ini','w') as configfile:
		config.write(configfile)
	IOTAAddress = getAddress(addressIndex)

def getAddress (index):
	generator = AddressGenerator(seed)
	addresses = generator.get_addresses(start=index,count=1)
	for address in addresses:
		print(address)
	return(address)
	
def writeTag(consignmentID):
	try:
		#Initialize LED
#		GPGPIO.setmode(GPGPIO.BCM)
#		myled = LED(21)
#		myled.blink(n=10,background=1)
		print('Please place the tag on the reader')
		tagText = str(addressIndex) + ',' + consignmentID
		id,text=reader.write(tagText)
		print("Written")
		print(id)
#		myled.on()
#		sleep(2)
#		myled.off()
		GPIO.setmode (GPIO.BCM)      
		GPIO.setup(21,GPIO.OUT)             
		GPIO.output(21,1)                     
		sleep(1)             
		GPIO.cleanup()  
	finally:
		GPIO.cleanup()
	return (int(id),int(addressIndex))
def readTag():
	try:
		#Initialize LED
#		GPGPIO.setmode(GPGPIO.BCM)
#		myled = LED(21)
#		myled.blink(n=10,background=1)
		print('Please place the tag on the reader')
		id, text = reader.read()
		addIndex,consID=text.split(",")
#		myled.on()
#		sleep(2)
#		myled.off()
		GPIO.setmode (GPIO.BCM)      
		GPIO.setup(21,GPIO.OUT)             
		GPIO.output(21,1)                     
		sleep(1)             
		GPIO.cleanup()               

	finally:
		GPIO.cleanup()
	return(id,addIndex,consID)
		
def newConsignment(consid = ''):
	global consignmentID
	print('New consignment tracking being generated.')
	if interface  == 'cli':
		consignmentID = raw_input("\nPlease enter the consignment ID and press enter:")
		confirmDelete = raw_input("\nKeep in mind that the existing data on the RFID will be deleted. press y to continue\n")
		if confirmDelete != 'y':
			return()
	else:
		consignmentID = consid
	addid=generateAddress()
	print('address generated')
	rfid,addix=writeTag(consignmentID)
	print('tag written')
	signer= "newTrack"
	rfid,rfterminal,cons,iotadd,signed=trackConsignment(signer,rfid,addix,consignmentID)
	
	return(rfid,rfterminal,cons,iotadd,signed)

def signConsignment():
	signedBy = raw_input("\nPlease enter your name for the sign-off: ")
	trackConsignment(signedBy)

def trackConsignment(signer = '',rfid = '',addix = '', cID = ''):
	
	try:
		if rfid >= 0 and cID and addix > 0 :
			id = rfid
			consID = cID
			addIndex = addix
		else:
			id,addIndex,consID = readTag()
		IOTAAddress = getAddress(int(addIndex))
		print "Adding tracking to ",IOTAAddress

		data = {'tagID': str(id), 'terminal': terminal, 'consignmentID': str(consID), 'Signatory': str(signer)}

		pt = iota.ProposedTransaction(address = iota.Address(IOTAAddress),
									  message = iota.TryteString.from_unicode(json.dumps(data)),
									  tag     = iota.Tag(b'BFIOTA'),
									  value   = 0)

		print("\nID card detected...Sending transaction...Please wait...")

		FinalBundle = api.send_transfer(depth=3, transfers=[pt], min_weight_magnitude=14)['bundle']

		print("\nTransaction sucessfully completed, have a nice day")
		
	except KeyboardInterrupt:
		print("cleaning up")
        scan_time = datetime.now().strftime('%Y-%m-%d %H:%M:%S')
        data['scan_time'] = scan_time
	message = json.dumps(data)
	if callHookNew > 0 and str(signer) == 'newTrack':
		sendProxeus(message,documentIDnew)
	elif callHookSign > 0 and  str(signer) != "":
		sendProxeus(message,documentIDsign)
	elif callHookTrack > 0:
		sendProxeus(message,documentIDtrack)
	return(str(id),terminal,str(consID),str(IOTAAddress), str(signer))
		

def showConsignment(IOTAAddress = ''):
	txcache ={}
	json_data =''
	json_api = ''
	if not IOTAAddress:
		id,addIndex,consID = readTag()
		IOTAAddress = getAddress(int(addIndex))
#	IOTAAddress = getAddress(9)
	print ("loading pickle")
	if os.path.isfile("txcache"+str(IOTAAddress)+".p"):
		txcache = pickle.load( open( "txcache"+str(IOTAAddress)+".p", "rb"))
	print IOTAAddress
	x = PrettyTable()
	x.field_names = ["tagID", "Terminal", "ConsignmentID", "scan_time", "Signatory"]
	result = api.find_transactions(addresses=[IOTAAddress])
	myhashes = result['hashes']
	print("Please wait while retrieving records from the tangle...")
#	print result
	json_api=[]
	for txn_hash in myhashes:
		txn_hash_as_bytes = bytes(txn_hash)
#		print (txn_hash_as_bytes)
		if not txn_hash_as_bytes in txcache:
			gt_result = api.get_trytes([txn_hash_as_bytes])
			trytes = str(gt_result['trytes'][0])
			txn = Transaction.from_tryte_string(trytes)
			timestamp = txn.timestamp
			scan_time = datetime.fromtimestamp(timestamp).strftime('%Y-%m-%d %H:%M:%S')
			txn_data = str(txn.signature_message_fragment.decode())
			json_data = json.loads(txn_data)
			if not 'Signatory' in json_data:
				json_data['Signatory'] = ''
			json_data['scan_time'] = scan_time
			txcache[txn_hash_as_bytes]=json_data
			pickle.dump( txcache,open( "txcache"+str(IOTAAddress)+".p", "wb" ))
		else:
			json_data = txcache[txn_hash_as_bytes]
		json_api.append(json_data)
		if all(key in json.dumps(json_data) for key in ["tagID","terminal","consignmentID"]):
			x.add_row([json_data['tagID'], json_data['terminal'], json_data['consignmentID'], json_data['scan_time'], json_data['Signatory']])

	x.sortby = "scan_time"

	print(x)

	pickle.dump( txcache,open( "txcache"+str(IOTAAddress)+".p", "wb" ))
	return(json.dumps(json_api))

@app.route('/track')	
def apiTrack():
	rfid,rfterminal,cons,iotadd,signed=trackConsignment()
	return json_response(id = rfid, terminal = rfterminal, consignment = cons, iotaddress = iotadd, signer = signed)

@app.route('/sign', methods=['POST'])	
def apiSign():
	data = request.get_json(force=True)
	try:
		signer = data['signer']
	except (KeyError, TypeError, ValueError):
		raise JsonError(description='Invalid value.')
	rfid,rfterminal,cons,iotadd,signed=trackConsignment(signer)
	return json_response(id = rfid, terminal = rfterminal, consignment = cons, iotaddress = iotadd, signer = signed)

@app.route('/newtrack', methods=['POST'])	
def apiNewTrack():
	data = request.get_json(force=True)
	try:
		consignmentID = data['consignmentID']
	except (KeyError, TypeError, ValueError):
		raise JsonError(description='Invalid value.')
	rfid,rfterminal,cons,iotadd,signed=newConsignment(consignmentID)
	return json_response(id = rfid, terminal = rfterminal, consignment = consignmentID, iotaddress = iotadd, signer = signed)

@app.route('/show', methods=['POST'])	
def apiShowConsignment():
	data = request.get_json(force=True)
	try:
		iotadd = data['iotaddress']
	except (KeyError, TypeError, ValueError):
		raise JsonError(description='Invalid value.')
	#print showConsignment(iotadd)
	return showConsignment(iotadd)
	
def functionMenu ():
	# Show welcome message

	print("\nWelcome to the Blockfactory Tracking System")
	print("Press Ctrl+C to exit the system")
	print("\n\nWould you like to:\n 1. Track consignment\n 2. Sign receipt for a consignment\n 3. Open up a new tracking\n 4. Set terminal in monitoring mode\n 5. Display tracking details\n")
	command = input("\nPlease choose and press Enter: ")

	if command == 1:
		trackConsignment()
	elif command == 2:
		signConsignment()
	elif command == 3:
		newConsignment()
	elif command == 4:
		while True:
			trackConsignment()
	elif command == 5:
		showConsignment()
        elif command == 6:
		for i in range(1,addressIndex):
 	               showConsignment(getAddress(i))


if interface == 'api':
	if __name__ == '__main__':
		app.run(host='0.0.0.0')
else:
	while True:		
		functionMenu()
