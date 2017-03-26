/**
 * Created by slanders on 3/12/17.
 */
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


let hfc = require('fabric-client');
let utils = require('fabric-client/lib/utils.js');
let Peer = require('fabric-client/lib/Peer.js');
let Orderer = require('fabric-client/lib/Orderer.js');
let EventHub = require('fabric-client/lib/EventHub.js');

let config = require('./config.json');
let helper = require('./helper.js');

let client = new hfc();
let chain;

let orderer = process.env.ORDERER_GRPC_URL;
let peers = process.env.PEER_GRPC_URLS.split(",");
let eventhubUrl = process.env.EVENT_HUB_URL;


let voteEventHub = new EventHub();
voteEventHub.setPeerAddr(eventhubUrl);
voteEventHub.connect();
voteEventHub.registerChaincodeEvent(config.chaincodeID, "VOTE", (voteEvent)=>{
    for(let key in voteEvent){
        console.log("EVENT: "+key+"="+voteEvent[key]);
    }
    console.log("EVENT JSON: "+voteEvent.encodeJSON());
    console.log("EVENT PAYLOAD"+voteEvent.getPayload().toString("utf8"));

    //TODO: submit to pub/sub
});

init();

function init() {
    chain = client.newChain(config.chainName);
    chain.addOrderer(new Orderer(orderer));
    for (let i = 0; i < peers.length; i++) {
        chain.addPeer(new Peer(peers[i]));
    }
}

module.exports.invoke = (func, jsonArg, commitHandler) => {
    return new Promise(function(resolve, reject) {
        let tx_id = null;
        let eventhub = new EventHub();
        eventhub.setPeerAddr(eventhubUrl);
        eventhub.connect();

        hfc.newDefaultKeyValueStore({
            path: config.keyValueStore
        }).then(function (store) {
            client.setStateStore(store);
            return helper.getSubmitter(client);
        }).then(
            function (admin) {
                tx_id = helper.getTxId();
                let nonce = utils.getNonce();
                let args = helper.getArgs([func, JSON.stringify(jsonArg), ""+Math.floor(new Date().getTime()/1000)]);

                let request = {
                    chaincodeId: config.chaincodeID,
                    fcn: "invoke",
                    args: args,
                    chainId: config.channelID,
                    txId: tx_id,
                    nonce: nonce
                };

                console.log("request=" + JSON.stringify(request))
                return chain.sendTransactionProposal(request);
            }
        ).then(
            function (results) {
                console.log('Obtained proposal responses from endorsers');
                let request = helper.processProposal(results, func);
                if(request.status === "success"){
                    resolve(request.proposalResponses[0].response.payload);
                    return helper.submitTransaction(request, chain);
                }else{
                    try {
                        let errorJson = request.message.substring(request.message.indexOf("{"));
                        let errorObj = JSON.parse(errorJson)
                        reject(errorObj);
                    }catch(e){
                        logger.error("cannot parse error:"+ request.message);
                        reject({Code: 500, Message: "Invalid Response"});
                    }
                }
            }
        ).then(
            function (response) {
                if (response.status === 'SUCCESS') {
                    let handle = setTimeout(() => {
                        console.error('Failed to receive transaction notification within the timeout period');
                        if (commitHandler !== undefined) {
                            commitHandler({
                                result: "failed"
                            });
                        }
                    }, parseInt(config.waitTime));

                    console.log("registering for events on "+tx_id);
                    eventhub.registerTxEvent(tx_id, (tx) => {
                        console.log("Transaction has been successfully committed");
                        clearTimeout(handle);
                        eventhub.disconnect();
                        if (commitHandler !== undefined) {
                            commitHandler({
                                result: "success"
                            });
                        }
                    });
                }
            }
        ).catch(
            function (err) {
                eventhub.disconnect();
                reject('Failed to invoke transaction due to error: ' + err.stack ? err.stack : err)
            }
        );
    });
};


