const $ = window.$;

console.log("test");

let remoteVideo = null;
let peerConnection = null;
let peerConnectionConfig = {
  iceServers: [],
};
let localStream = null;
let streamServer = "vrlive.newjivr.com";
let wsURL = `wss://${streamServer}/webrtc-session.json`;
let wsConnection = null;
let streamInfo = {
  applicationName: "live",
  streamName: "upload",
  sessionId: "[empty]",
};
let repeaterRetryCount = 0;
let newAPI = false;
let failedProfileIds = []

let overrideProfileId = true;

window.RTCPeerConnection = window.RTCPeerConnection ||
  window.mozRTCPeerConnection ||
  window.webkitRTCPeerConnection;
window.RTCIceCandidate = window.RTCIceCandidate ||
  window.mozRTCIceCandidate ||
  window.webkitRTCIceCandidate;
window.RTCSessionDescription = window.RTCSessionDescription ||
  window.mozRTCSessionDescription ||
  window.webkitRTCSessionDescription;


// UI
window.pageReady = function() {
  remoteVideo = document.getElementById("remoteVideo");
  if (navigator.mediaDevices.getUserMedia) {
    newAPI = false;
  }

  $("#custom-app-name").val(
    $("#agora-stream-data").attr("data-wowza-app") ||
    localStorage.getItem("custom-app") || "live-test"
  );
  $("#custom-stream-name").val(
    $("#agora-stream-data").attr("data-wowza-stream") ||
    localStorage.getItem("custom-stream") || "upload"
  );
  $("#custom-signalling-server").val(
    $("#agora-stream-data").attr("data-wowza-signalling") ||
    localStorage.getItem("custom-signalling-server") || wsURL
  );

  // $("#update-wowza-stream-status").on("click", () => {
  //   $("#wowza-stream-status").text("...")

  //   const app = $("#custom-app-name").val()
  //   const stream = $("#custom-stream-name").val()

  //   isStreamAvailable(app, stream, (status, data) => {
  //     console.log(status, data)
  //     $("#wowza-stream-status").text(status ? "配信" : "未配信")
  //   })
  // })
};

window.logAndSet = function(selector, name, value) {
  $(selector).text(value)
  const prev = $("#log").val()
  const newLine = (new Date()).toISOString() + ": " + name + ": " + value
  $("#log").val(prev + (prev ? "\n": "") + newLine)
}

window.logStats = async function() {
  // initGraph(peerConnection)

  // const lid = 3
  // const pid = 21250
  // const rid = 4869

  // window.addPeerConnection({
  //   connected: false,
  //   constraints: "",
  //   isOpen: true,
  //   lid,
  //   pid,
  //   rid,
  //   rtcConfiguration: JSON.stringify(),
  //   url: "http://localhost:3000/webrtc-player-test",
  // })

  // const rawStats = await peerConnection.getStats()
  // const stats = rawStats.map(s => {
  //   const {id, type, timestamp, ...values} = s[1]
  //   return {
  //     id,
  //     type,
  //     stats: {
  //       timestamp,
  //       values: Object.entries(values).flat()
  //     }
  //   }
  // })
  // window.addStandardStats({
  //   lid,
  //   pid,
  //   reports: stats,
  // })
}

function getConfig() {
  let config = JSON.parse(JSON.stringify(peerConnectionConfig));
  const forceRelay = $("#cb-relay").get()[0].checked;
  const useUDP = $("#cb-udp").get()[0].checked;
  const useTCP = $("#cb-tcp").get()[0].checked;
  const useTLS = $("#cb-tls").get()[0].checked;
  const use443 = $("#cb-443").get()[0].checked;
  const useExtra = $("#cb-extra").get()[0].checked;
  const useLocalhost = $("#cb-local").get()[0].checked;
  // console.log(forceRelay, useTCP, useUDP);

  const domain = useLocalhost ? "localhost" : "stg-service.newjivr.com"

  console.log("forceRelay, useUDP, useTCP, useTLS, use443, useExtra:", forceRelay, useUDP, useTCP, useTLS, use443, useExtra)

  if (forceRelay) {
    config.iceTransportPolicy = "relay" // TURN 強制
  }

  if (use443) {
    config.iceServers.push({
      urls: `turn:${domain}:443?transport=udp`,
      username: "username",
      credential: "passworddayo",
    })
    config.iceServers.push({
      urls: `turn:${domain}:443?transport=tcp`,
      username: "username",
      credential: "passworddayo",
    })
    config.iceServers.push({
      urls: `turns:${domain}:443?transport=tcp`,
      username: "username",
      credential: "passworddayo",
    })
  }

  if (useExtra) {
    config.iceServers.push({
      urls: `turn:${domain}:3478?transport=udp`,
      username: "username",
      credential: "passworddayo",
    })
    config.iceServers.push({
      urls: `turn:${domain}:3478?transport=tcp`,
      username: "username",
      credential: "passworddayo",
    })
    config.iceServers.push({
      urls: `turns:${domain}:5349?transport=tcp`,
      username: "username",
      credential: "passworddayo",
    })
    // config.iceServers.push({
    //   urls: 'turns:stg-service.newjivr.com:1935?transport=tcp",
    //   username: "username",
    //   credential: "passworddayo",
    // })
  }

  if (!useUDP) {
    config.iceServers = config.iceServers.filter((v) =>
      !v.urls.includes("transport=udp")
    );
  }
  if (!useTCP) {
    config.iceServers = config.iceServers.filter((v) =>
      !(v.urls.includes("transport=tcp") && !v.urls.includes("turns:"))
    );
  }
  if (!useTLS) {
    config.iceServers = config.iceServers.filter((v) =>
      !v.urls.includes("turns:")
    );
  }

  return config
}


// interface
window.start = function() {
  if (peerConnection == null) {
    startPlay();
  } else {
    stopPlay();
  }
};

window.reloadScreen = function() {
  stopPlay();
  startPlay();
};

window.startPlay = function() {
  repeaterRetryCount = 0;

	failedProfileIds = []

  // ログ削除
  logAndSet("#turn-connection-state", "")
  logAndSet("#connection-state", "")
  logAndSet("#ice-connection-state", "")
  logAndSet("#ice-gathering-state", "")
  logAndSet("#signaling-state", "")
  logAndSet("#wowza-stream-status", "")
  logAndSet("#wowza-stream-status", "")
  logAndSet("#wowza-stream-status", "")
  logAndSet("#wowza-stream-status", "")
  logAndSet("#wowza-stream-status", "")
  logAndSet("#offer-profile", "")
  logAndSet("#offer-ice-candidates", "")
  logAndSet("#answer-profile", "")

	// 配信設定
  streamInfo.applicationName = $("#custom-app-name").val();
  streamInfo.streamName = $("#custom-stream-name").val();
  const customSignallingServer = $("#custom-signalling-server").val();

  localStorage.setItem("custom-app", $("#custom-app-name").val());
  localStorage.setItem("custom-stream", $("#custom-stream-name").val());
  localStorage.setItem("custom-signalling-server", $("#custom-signalling-server").val());

  overrideProfileId = $("#cb-profile-override").get()[0].checked;

  let config = getConfig()
  console.log(config);

	// 接続
  let ws = customSignallingServer || wsURL
  console.log(
    "startPlay: wsURL:" + ws + " streamInfo:" + JSON.stringify(streamInfo),
  );
  wsConnect(ws, config);
};

window.stopPlay = function() {
  if (peerConnection != null) {
    peerConnection.close();
    peerConnection = null;
  }

  if (wsConnection != null) {
    wsConnection.close();
    wsConnection = null;
  }

  remoteVideo.src = ""; // this seems like a chrome bug - if set to null it will make HTTP request

  console.log("stopPlay");
};

window.isConnected = function() {
  return peerConnection;
};


window.checkTurn = function() {
  let begin
  let candidates = []
  let config = getConfig()

  let pc = new RTCPeerConnection(config);
  pc.onicecandidate = iceCallback;
  pc.onicegatheringstatechange = gatheringStateChange;
  // pc.onicecandidateerror = iceCandidateError;
  pc.createOffer(
    {offerToReceiveAudio: 1}
  ).then(
    gotDescription,
    noDescription
  );

  $("#turn-connection-state").text("...")

  function iceCallback(event) {
    const elapsed = ((window.performance.now() - begin) / 1000).toFixed(3);
    if (event.candidate) {
      if (event.candidate.candidate === '') {
        return;
      }
      const {candidate} = event;
      // candidate.type
      candidates.push(candidate);
    } else if (!('onicegatheringstatechange' in RTCPeerConnection.prototype)) {
      // should not be done if its done in the icegatheringstatechange callback.
      pc.close();
      pc = null;
    }
  }

  function gatheringStateChange() {
    if (pc.iceGatheringState === 'complete') {
      let found = candidates.find(v => v.type === "relay")
      logAndSet("#turn-connection-state", "TURN server state", found ? "接続成功" : "接続失敗")
    } else {
      console.debug("finish", candidates)
      return;
    }
    const elapsed = ((window.performance.now() - begin) / 1000).toFixed(3);
    pc.close();
    pc = null;
  }

  function gotDescription(desc) {
    begin = window.performance.now();
    candidates = [];
    pc.setLocalDescription(desc);
  }

  function noDescription(error) {
    console.error('Error creating offer: ', error);
  }

}

window.isStreamAvailable = function(applicationName, streamName, callback, errCallback) {
  const streamInfo = {
    streamName: streamName,
    applicationName: applicationName,
  };

  let wsConn = new WebSocket(wsURL);
  wsConn.binaryType = "arraybuffer";

  wsConn.onopen = function () {
    wsConn.send(
      '{"direction":"play", "command":"getAvailableStreams", "streamInfo":' +
        JSON.stringify(streamInfo) +
        "}",
    );
  };

  wsConn.onmessage = function (evt) {
    wsConn.close();
    wsConn = null;

    let msgJSON = JSON.parse(evt.data);
    let status = msgJSON["status"];
    let streams = msgJSON["availableStreams"];

    let available = streams && streams.find((v) => v.streamName == streamName);

    callback(available, msgJSON);
  };

  // wsConnection.onclose = function()
  // {
  // 	console.log("wsConnection.onclose");
  // }
  wsConnection.onerror = function(evt)
  {
    errCallback(evt)
  }
};

window.wsConnect = function(url, config) {
  stopPlay();

  wsConnection = new WebSocket(url);
  wsConnection.binaryType = "arraybuffer";

  wsConnection.onopen = function () {
    console.log("wsConnection.onopen");

    peerConnection = new RTCPeerConnection(config);
    peerConnection.onicecandidate = gotIceCandidate;
    peerConnection.onconnectionstatechange = e => {
      console.log("onconnectionstatechange:", peerConnection.connectionState, e)
      logAndSet("#connection-state", "connection state", peerConnection.connectionState)
    }
    peerConnection.oniceconnectionstatechange = e => {
      console.log("oniceconnectionstatechange:", peerConnection.iceConnectionState, e)
      logAndSet("#ice-connection-state", "ICE connection state", peerConnection.iceConnectionState)
    }
    peerConnection.onicegatheringstatechange = e => {
      console.log("onicegatheringstatechange:", peerConnection.iceGatheringState, e)
      logAndSet("#ice-gathering-state", "ICE gathering state", peerConnection.iceGatheringState)
    }
    peerConnection.onsignalingstatechange = e => {
      console.log("onsignalingstatechange:", peerConnection.signalingState, e)
      logAndSet("#signaling-state", "signaling state", peerConnection.signalingState)
    }

    if (newAPI) {
      peerConnection.ontrack = gotRemoteTrack;
    } else {
      peerConnection.onaddstream = gotRemoteStream;
    }

    $("#wowza-stream-status").text("")
    console.log("wsURL: " + wsURL);
    sendPlayGetOffer();
  };

  function sendPlayGetOffer() {
    console.log("sendPlayGetOffer: " + JSON.stringify(streamInfo));
    wsConnection.send(
      '{"direction":"play", "command":"getOffer", "streamInfo":' +
        JSON.stringify(streamInfo) +
        "}",
    );
  }

  wsConnection.onmessage = function (evt) {
    console.log("wsConnection.onmessage: " + evt.data);
    let msgJSON = JSON.parse(evt.data);

    let msgStatus = Number(msgJSON["status"]);
    let msgCommand = msgJSON["command"];

    if (msgStatus == 514) {
      logAndSet("#wowza-stream-status", "Wowza stream status", "repeater stream not ready")
      console.log("Wowza WebRTC response: repeater stream not ready");
      // repeater stream not ready
      repeaterRetryCount++;
      if (repeaterRetryCount < 10) {
        setTimeout(sendGetOffer, 500);
      } else {
        console.log("Live stream repeater timeout: " + streamName);
        stopPlay();
      }
    } else if (msgStatus == 502) {
      logAndSet("#wowza-stream-status", "Wowza stream status", "未配信")
      console.log("Wowza WebRTC response: まだ配信されていません");
      stopPlay();
    } else if (msgStatus == 519) {
      logAndSet("#wowza-stream-status", "Wowza stream status", "認証失敗")
      alert("認証に失敗しました");
      stopPlay();
    } else if (msgStatus != 200) {
      logAndSet("#wowza-stream-status", "Wowza stream status", "エラー:" + JSON.stringify(msgStatus))
      console.log("Wowza WebRTC response: status " + msgStatus);

      stopPlay();
    } else {
      logAndSet("#wowza-stream-status", "Wowza stream status", "配信中")

      let streamInfoResponse = msgJSON["streamInfo"];
      if (streamInfoResponse !== undefined) {
        streamInfo.sessionId = streamInfoResponse.sessionId;
      }

      let sdpData = msgJSON["sdp"];
      if (sdpData !== undefined) {
        // console.log('sdp: '+JSON.stringify(sdpData));

        if (sdpData.type == "offer") {
          // We mundge the SDP here, before creating an Answer
          // If you can get the new MediaAPI to work this might
          // not be needed.
          const match = sdpData.sdp.match(/profile-level-id=(\w+);/)
          console.log("overrideProfileId:", overrideProfileId)
          if (match && overrideProfileId) {
            sdpData.sdp = enhanceSDP(sdpData.sdp);
          }
          const profile = sdpData.sdp.match(/profile-level-id=(\w+);/);
          if (match && profile) {
            logAndSet("#offer-profile", "offer profile-level-id", match[1] + "->" + profile[1])
          }
          console.log("offer:", sdpData);

          peerConnection.setRemoteDescription(new RTCSessionDescription(sdpData)).then(() => {
            peerConnection.createAnswer().then(gotDescription).catch(errorHandler);
          }).catch((err) => {
            console.log("setRemoteDescription error:", err);
          })
        } else if (sdpData.ice) {
          console.log("ice: " + JSON.stringify(sdpData.ice));
          peerConnection.addIceCandidate(new RTCIceCandidate(sdpData.ice));
        }
      }

      let iceCandidates = msgJSON["iceCandidates"];
      if (iceCandidates !== undefined) {
        console.log("iceCandidates: " + JSON.stringify(iceCandidates));
        logAndSet("#offer-ice-candidates", "offer ICE candidate", iceCandidates.length + " candidates")
        for (let index in iceCandidates) {
          peerConnection.addIceCandidate(
            new RTCIceCandidate(iceCandidates[index]),
          );
        }
      }
    }

    if ("sendResponse".localeCompare(msgCommand) == 0) {
      console.log("Wowza WebRTC response: received sendReponse, closing");
      if (wsConnection != null) wsConnection.close();
      wsConnection = null;
    }
  };

  wsConnection.onclose = function () {
    console.log("wsConnection.onclose");
  };

  wsConnection.onerror = function (evt) {
    console.log("wsConnection.onerror: " + JSON.stringify(evt));
  };
};

function getNextProfile() {
}

window.enhanceSDP = function(sdpStr) {
  let sdpLines = sdpStr.split(/\r\n/);
  let sdpSection = "header";
  let hitMID = false;
  // let sdpStrRet = "";

  // for (let sdpIndex in sdpLines) {
  //   let sdpLine = sdpLines[sdpIndex];

  //   if (sdpLine.length == 0) continue;

  //   if (sdpLine.includes("profile-level-id")) {
  //     console.log("found profile-id");
  //     // This profile seems to be correct for the stream publishing,
  //     // however will not allow Safari to play it back, so we swap
  //     // it for a baseline constrained one, which is declared when
  //     // Safari publishes in the SDP.
  //     sdpLine = sdpLine.replace(/640029/ig, "42E01F");

  //     sdpLine = sdpLine.replace(/4d0033/ig, "42c033");
  //   }

  //   sdpStrRet += sdpLine;
  //   sdpStrRet += "\r\n";
  // }

  // Look for codecs higher than baseline and force downward.
  // https://github.com/koala-interactive/wowza-webrtc-player/blob/2a74907d1f26278df140171f5935160dc458d97a/src/webrtc/SDPEnhancer.ts#L12
  let sdpStrRet = sdpStr.replace(
    /profile-level-id=(\w+);/gi,
    (_, orig) => {
      const profileId = parseInt(orig, 16);
      let profile = (profileId >> 16) & 0xff;
      let constraint = (profileId >> 8) & 0xff;
      let level = profileId & 0xff;

      if (profile > 0x42) {
        profile = 0x42;
        constraint = 0xe0;
        level = 0x1f;
      } else if (constraint === 0x00) {
        constraint = 0xe0;
      }

      return "profile-level-id=" + ((profile << 16) | (constraint << 8) | level).toString(16) + ";";
    }
  );

  console.log("Resuling SDP: " + sdpStrRet);
  return sdpStrRet;
};

window.gotDescription = function(description) {
  console.log(
    "gotDescription: created answer:",
    description,
  );
  const profile = description.sdp.match(/profile-level-id=(\w+);/)
  console.log(profile)
  logAndSet("#answer-profile", "answer profile-level-id", profile && profile[1])

  peerConnection.setLocalDescription(description).then(() => {
		console.log("sendAnswer");
		wsConnection.send(
			'{"direction":"play", "command":"sendResponse", "streamInfo":' +
				JSON.stringify(streamInfo) +
				', "sdp":' +
				JSON.stringify(description) +
				"}",
		);
	}).catch(() => {
		console.log("set description error");
	})
};

window.gotIceCandidate = function(event) {
  console.log("gotIceCandidate:", event);
  if (event.candidate != null) {
  }
};

window.gotRemoteTrack = function(event) {
  console.log(
    "gotRemoteTrack: kind:" + event.track.kind + " stream:" + event.streams[0],
  );
  try {
    remoteVideo.srcObject = event.streams[0];
  } catch (error) {
    remoteVideo.src = window.URL.createObjectURL(event.streams[0]);
  }
};

window.gotRemoteStream = function(event) {
  console.log("gotRemoteStream: " + event.stream);
  try {
    remoteVideo.srcObject = event.stream;
  } catch (error) {
    remoteVideo.src = window.URL.createObjectURL(event.stream);
  }
};

window.errorHandler = function(error) {
  console.log(error);
};
