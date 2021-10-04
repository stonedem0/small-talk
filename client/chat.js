let ws  = new WebSocket('ws://' + window.location.host + '/ws');


// const form = document.getElementById('form');


const sendMessage = event => {
    const usersname = document.getElementById('username').value;
    const messsage = document.getElementById('message').value
    ws.send(JSON.stringify({
        username: usersname,
        message: messsage
    }))
    event.preventDefault();
}
 

const submit = document.getElementById('submit');
submit.addEventListener('submit', sendMessage);