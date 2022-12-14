/*
 * ************************** BEGIN LICENSE BLOCK ******************************
 *
 * Copyright © 2023 Christian Stolze
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * ************************** END LICENSE BLOCK ********************************
 */

package main

/*

This file contains all functions and other declarations for the service app.

*/

import (

  // standard
  "fmt"
  // "os"
  "time"
  "sync"

  // external
  hederasdk "github.com/hashgraph/hedera-sdk-go/v2"

  // internal
  . "renderhive/constants"
  "renderhive/logger"
  //"renderhive/cli"
  "renderhive/node"
  "renderhive/hedera"
  "renderhive/ipfs"
  "renderhive/renderer"
  "renderhive/webapp"
)



// STRUCTURES
// #############################################################################
// Data required to manage the nodes
type ServiceApp struct {

  // Managers
  NodeManager *node.NodeManager
  HederaManager *hedera.HederaManager
  IPFSManager *ipfs.IPFSManager
  RenderManager *renderer.RenderManager
  WebAppManager *webapp.WebAppManager

  // Hedera consensus service topics
  // Hive cycle topics
  HiveCycleSynchronizationTopic hedera.HederaTopic
  HiveCycleApplicationTopic hedera.HederaTopic
  HiveCycleValidationTopic hedera.HederaTopic

  // Render job topics
  JobQueueTopic hedera.HederaTopic
  JobTopics []hedera.HederaTopic

  // Signaling channels
  Quit chan bool
  WG sync.WaitGroup

}


// FUNCTIONS
// #############################################################################
// Initialize the Renderhive Service App session
func (service *ServiceApp) Init() (error) {
    var err error
    var topic *hedera.HederaTopic

    // log the start of the renderhive service
    logger.RenderhiveLogger.Main.Info().Msg("Starting Renderhive service app.")

    // INITIALIZE INTERNAL MANAGERS
    // *************************************************************************
    // initialize the node manager
    service.NodeManager = &node.NodeManager{}
    err = service.NodeManager.Init()
    if err != nil {
      return err
    }

    // initialize the Hedera manager
    service.HederaManager = &hedera.HederaManager{}
    err = service.HederaManager.Init(hedera.NETWORK_TYPE_TESTNET, "hedera/testnet.env")
    if err != nil {
      return err
    }
    logger.RenderhiveLogger.Main.Info().Msg("Loaded the account details from the environment file.")
    logger.RenderhiveLogger.Main.Info().Msg(fmt.Sprintf(" [#] Public key: %s", service.HederaManager.Operator.PublicKey))
    logger.RenderhiveLogger.Main.Info().Msg(fmt.Sprintf("Mirror node: %v", service.HederaManager.MirrorNode.URL))

    // initialize the IPFS manager
    service.IPFSManager = &ipfs.IPFSManager{}
    err = service.IPFSManager.Init()
    if err != nil {
      return err
    }

    // initialize the render manager
    service.RenderManager = &renderer.RenderManager{}
    err = service.RenderManager.Init()
    if err != nil {
      return err
    }

    // initialize the web app manager
    service.WebAppManager = &webapp.WebAppManager{}
    err = service.WebAppManager.Init()
    if err != nil {
      return err
    }

    // READ HCS TOPIC INFORMATION & SUBSCRIBE
    // *************************************************************************
    // render job queue
    topic, err = service.HederaManager.TopicInfoFromString(RENDERHIVE_TESTNET_RENDER_JOB_QUEUE)
    if err != nil {
      return err
    }
    err = service.HederaManager.TopicSubscribe(topic, time.Unix(0, 0), func(message hederasdk.TopicMessage) {

      logger.RenderhiveLogger.Package["hedera"].Info().Msg(fmt.Sprintf("Message received: %s", string(message.Contents)))

    })
    if err != nil {
      return err
    }

    // hive cycle synchronization topic
    topic, err = service.HederaManager.TopicInfoFromString(RENDERHIVE_TESTNET_TOPIC_HIVE_CYCLE_SYNCHRONIZATION)
    if err != nil {
      return err
    }
    err = service.HederaManager.TopicSubscribe(topic, time.Unix(0, 0), service.NodeManager.HiveCycle.MessageCallback())
    if err != nil {
      return err
    }

    // hive cycle application topic
    topic, err = service.HederaManager.TopicInfoFromString(RENDERHIVE_TESTNET_TOPIC_HIVE_CYCLE_APPLICATION)
    if err != nil {
      return err
    }
    err = service.HederaManager.TopicSubscribe(topic, time.Unix(0, 0), func(message hederasdk.TopicMessage) {

      logger.RenderhiveLogger.Package["hedera"].Info().Msg(fmt.Sprintf("Message received: %s", string(message.Contents)))

    })
    if err != nil {
      return err
    }

    // hive cycle validation topic
    topic, err = service.HederaManager.TopicInfoFromString(RENDERHIVE_TESTNET_TOPIC_HIVE_CYCLE_VALIDATION)
    if err != nil {
      return err
    }
    err = service.HederaManager.TopicSubscribe(topic, time.Unix(0, 0), func(message hederasdk.TopicMessage) {

      logger.RenderhiveLogger.Package["hedera"].Info().Msg(fmt.Sprintf("Message received: %s", string(message.Contents)))

    })
    if err != nil {
      return err
    }



    // HIVE CYCLE
    // *************************************************************************
    // synchronize with the render hive
    service.NodeManager.HiveCycle.Synchronize(service.HederaManager)


    go func() {

       // add call to wait group
       service.WG.Add(1)

       // loop forever
       for {

          select {

          // app is quitting
          case <-service.Quit:
            logger.RenderhiveLogger.Main.Debug().Msg("Stopped hive cycle synchronization loop.")
            service.WG.Done()
            return

          // app is running
          default:

            // synchronize the hive cycle
            service.NodeManager.HiveCycle.Synchronize(service.HederaManager)

            // get configuration and wait for the hive cycle duration
            configurations := service.NodeManager.HiveCycle.Configurations
            duration := time.Duration(configurations[len(configurations) - 1].Duration / 10)
            time.Sleep(duration * time.Second)
          }
       }
    }()



    // STATE CHECKS
    // *************************************************************************
    // perform important state checks
    // ...




    // LOG BASIC APP INFORMATION
    // *************************************************************************

    // log some informations about the used constants
    logger.RenderhiveLogger.Main.Info().Msg("This service app instance relies on the following smart contract(s) and HCS topic(s):")
    // the renderhive smart contract this instance calls
    logger.RenderhiveLogger.Main.Info().Msg(fmt.Sprintf(" [#] Smart Contract: %s", RENDERHIVE_TESTNET_SMART_CONTRACT))
    // Hive cycle
    logger.RenderhiveLogger.Main.Info().Msg(fmt.Sprintf(" [#] Hive Cycle Synchronization Topic: %s", RENDERHIVE_TESTNET_TOPIC_HIVE_CYCLE_SYNCHRONIZATION))
    logger.RenderhiveLogger.Main.Info().Msg(fmt.Sprintf(" [#] Hive Cycle Application Topic: %s", RENDERHIVE_TESTNET_TOPIC_HIVE_CYCLE_APPLICATION))
    logger.RenderhiveLogger.Main.Info().Msg(fmt.Sprintf(" [#] Hive Cycle Validation Topic: %s", RENDERHIVE_TESTNET_TOPIC_HIVE_CYCLE_VALIDATION))
    // Render jobs
    logger.RenderhiveLogger.Main.Info().Msg(fmt.Sprintf(" [#] Render Job Topic: %s", RENDERHIVE_TESTNET_TOPIC_HIVE_CYCLE_VALIDATION))


    return nil
}

// Deinitialize the Renderhive Service App session
func (service *ServiceApp) DeInit() (error) {
    var err error

    // log event
    logger.RenderhiveLogger.Main.Info().Msg("Stopping Renderhive service app ... ")

    // send the Quit signal to all concurrent go functions
    service.Quit <- true

    // log event
    logger.RenderhiveLogger.Main.Info().Msg("Waiting for background operations to shut down ... ")
    service.WG.Wait()

    // DEINITIALIZE INTERNAL MANAGERS
    // *************************************************************************

    // deinitialize the web app manager
    err = service.WebAppManager.DeInit()
    if err != nil {
      return err
    }

    // deinitialize the render manager
    err = service.RenderManager.DeInit()
    if err != nil {
      return err
    }

    // deinitialize the IPFS manager
    service.IPFSManager.DeInit()
    if err != nil {
      return err
    }

    // deinitialize the Hedera manager
    err = service.HederaManager.DeInit()
    if err != nil {
      return err
    }

    // deinitialize the node manager
    err = service.NodeManager.DeInit()
    if err != nil {
      return err
    }



    // LOG BASIC APP INFORMATION
    // *************************************************************************

    logger.RenderhiveLogger.Main.Info().Msg("Renderhive service app stopped.")

    return err

}
