/**
 * Copyright 2016 IBM All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */
// This is Sample end-to-end standalone program that focuses on exercising all
// parts of the fabric APIs in a happy-path scenario
'use strict';

let log4js = require('log4js');
let logger = log4js.getLogger('INVOKE');

let hfc = require('fabric-client');
let utils = require('fabric-client/lib/utils.js');
let Peer = require('fabric-client/lib/Peer.js');
let Orderer = require('fabric-client/lib/Orderer.js');
let EventHub = require('fabric-client/lib/EventHub.js');

let config = require('./config.json');
let helper = require('./helper.js');

logger.setLevel('DEBUG');

let client = new hfc();
let chain;
let eventhub;
let tx_id = null;

init();

function init() {
	chain = client.newChain(config.chainName);
	chain.addOrderer(new Orderer(config.orderer.orderer_url));
	eventhub = new EventHub();
	eventhub.setPeerAddr(config.events[0].event_url);
	eventhub.connect();
	for (let i = 0; i < config.peers.length; i++) {
		chain.addPeer(new Peer(config.peers[i].peer_url));
	}
}

hfc.newDefaultKeyValueStore({
	path: config.keyValueStore
}).then(function(store) {
	client.setStateStore(store);
	return helper.getSubmitter(client);
}).then(
	function(admin) {
		logger.info('Successfully obtained user to submit transaction');

		logger.info('Executing Invoke');
		tx_id = helper.getTxId();
		let nonce = utils.getNonce();
		let args = helper.getArgs(["add_ballot", JSON.stringify(config.createBallotData)]);
		// send proposal to endorser
		let request = {
			chaincodeId: config.chaincodeID,
			fcn: "invoke",
			args: args,
			chainId: config.channelID,
			txId: tx_id,
			nonce: nonce
		};
		logger.info("request="+JSON.stringify(request))
		return chain.sendTransactionProposal(request);
	}
).then(
	function(results) {
		logger.info('Successfully obtained proposal responses from endorsers');

		return helper.processProposal(chain, results, 'create_ballot');
	}
).then(
	function(response) {
		if (response.status === 'SUCCESS') {
			let handle = setTimeout(() => {
				logger.error('Failed to receive transaction notification within the timeout period');
				process.exit(1);
			}, parseInt(config.waitTime));

			eventhub.registerTxEvent(tx_id.toString(), (tx) => {
				logger.info('The chaincode transaction has been successfully committed');
				clearTimeout(handle);
				eventhub.disconnect();
			});
		}
	}
).catch(
	function(err) {
		eventhub.disconnect();
		logger.error('Failed to invoke transaction due to error: ' + err.stack ? err.stack : err);
	}
);
