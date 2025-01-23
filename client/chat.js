const ws = new WebSocket("ws://" + window.location.host + "/ws");

const submit = document.getElementById("submit");

const randomAnimals = ["Elephant", "Capybara", "Rat"];
const randomAdjective = ["shy", "cagy", "sneaky"];

window.addEventListener("DOMContentLoaded", () => {
  const storedUsername = localStorage.getItem("username");
  if (storedUsername) {
    document.getElementById("popup-overlay").style.display = "none";
    document.getElementById("chat-container").style.display = "flex";
  } else {
    document.getElementById("popup-overlay").style.display = "flex";
    document.getElementById("chat-container").style.display = "none";
  }
});

const chooseUsername = () => {
  animalLength = randomAnimals.length;
  adjectiveLength = randomAdjective.length;
  animalIndex = Math.floor(Math.random() * animalLength);
  adjectiveIndex = Math.floor(Math.random() * adjectiveLength);
  username = randomAdjective[adjectiveIndex] + randomAnimals[animalIndex];
  return username;
};

const signIn = () => {
  const username = document.getElementById("username-input").value.trim();
  if (username) {
    document.getElementById("popup-overlay").style.display = "none";
    document.getElementById("chat-container").style.display = "flex";
    localStorage.setItem("username", username);
    const storedUsername = localStorage.getItem("username");
    console.log("Stored username is:", storedUsername);
  } else {
    alert("please enter a valid username.");
  }
};

ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);
  const historyDiv = document.getElementById("messages");
  const p = document.createElement("p");
  p.textContent = `${msg.username}: ${msg.message}`;
  historyDiv.appendChild(p);
  historyDiv.scrollTop = historyDiv.scrollHeight;
};

ws.onopen = async () => {
  const response = await fetch("./history");
  if (!response.ok) {
    console.error("History fetch error:", response.statusText);
    return;
  }
  const data = await response.json();
  data.reverse(); // if data was newest->oldest, now it's oldest->newest
  const historyDiv = document.getElementById("messages");
  historyDiv.innerHTML = "";
  data.forEach((msgObj) => {
    const p = document.createElement("p");
    p.style.margin = "1px 1px 1px 1px";
    p.textContent = `${msgObj.username}: ${msgObj.message}`;
    historyDiv.appendChild(p);
  });
};

const sendMessage = (event) => {
  // localStorage.clear();
  const message = document.getElementById("message").value;
  // let username = document.getElementById("username").value;
  let username = localStorage.getItem("username");
  console.log("username", username);
  if (!username) {
    username = chooseUsername();
  }
  ws.send(
    JSON.stringify({
      username: username,
      message: message,
    })
  );
  event.preventDefault();
};

const displayMessages = (event) => {
  const data = JSON.parse(event.data);
  const chat = document.createElement("div");
  const currentDiv = document.getElementById("main");
  const username = document.createTextNode(`${data.username}: `);
  const message = document.createTextNode(data.message);
  document.body.insertBefore(chat, currentDiv);
  chat.appendChild(username);
  chat.appendChild(message);
  chat.classList.add("chat");
};

submit.addEventListener("submit", sendMessage);
