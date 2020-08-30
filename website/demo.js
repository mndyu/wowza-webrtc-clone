/* eslint-env browser */
var log = msg => {
  document.getElementById('logs').innerHTML += msg + '<br>'
}

document.addEventListener("DOMContentLoaded", async () => {
  const response = await fetch(`http://localhost:8080/rooms`);
  const data = await response.json()
  console.log(data)
  document.getElementById('rooms').innerHTML += data.map(v => {
    return `<p>room ${v.id}</p>`
  }).join("")
})

window.createSession = isPublisher => {
  let pc = new RTCPeerConnection({
    iceServers: [
      {
        urls: 'stun:stun.l.google.com:19302'
      }
    ]
  })
  pc.oniceconnectionstatechange = e => log(pc.iceConnectionState)
  pc.onicecandidate = event => {
    if (event.candidate === null) {
      document.getElementById('localSessionDescription').value = btoa(JSON.stringify(pc.localDescription))
    }
  }

  if (isPublisher) {
    navigator.mediaDevices.getUserMedia({ video: true, audio: false })
      .then(stream => {
        pc.addStream(document.getElementById('video1').srcObject = stream)
        pc.createOffer()
          .then(d => pc.setLocalDescription(d))
          .catch(log)
      }).catch(log)
  } else {
    pc.addTransceiver('video')
    pc.createOffer()
      .then(d => pc.setLocalDescription(d))
      .catch(log)

    pc.ontrack = function (event) {
      var el = document.getElementById('video1')
      el.srcObject = event.streams[0]
      el.autoplay = true
      el.controls = true
    }
  }

  window.startSession = async () => {
    let roomId = document.getElementById('roomId').value
    if (!isPublisher && roomId === '') {
      return alert('Room ID must not be empty')
    }
    let ld = document.getElementById('localSessionDescription').value
    if (ld === '') {
      return alert('Session Description must not be empty')
    }
    const path = isPublisher ? "call" : `${roomId}/recv`
    const response = await fetch(`http://localhost:8080/room/${path}`, {
      method: 'POST',
      body: ld
    });
    const sdp = await response.text()
    console.log("return value:", sdp)

    try {
      pc.setRemoteDescription(new RTCSessionDescription(JSON.parse(atob(sdp))))
    } catch (e) {
      alert(e)
    }
  }

  let btns = document.getElementsByClassName('createSessionButton')
  for (let i = 0; i < btns.length; i++) {
    btns[i].style = 'display: none'
  }

  document.getElementById('signalingContainer').style = 'display: block'
}
