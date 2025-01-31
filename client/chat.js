const ws = new WebSocket("ws://" + window.location.host + "/ws");

const submit = document.getElementById("submit");

const randomAnimals = ["Elephant", "Capybara", "Rat"];
const randomAdjective = ["shy", "cagy", "sneaky"];

let currentMessageColor = "black";
let currentMessageStyle = "normal";

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

const styleText = (command) => {
  let property;
  switch (command) {
    case "bold":
      property = "fontWeight";
      currentMessageStyle = "bold";
      break;
    case "italic":
      property = "fontStyle";
      currentMessageStyle = "italic";
      break;
    case "underline":
      property = "textDecoration";
      currentMessageStyle = "underline";
      break;
    case "line-through":
      console.log("strikethrough");
      property = "textDecoration";
      currentMessageStyle = "line-through";
      break;
  }
  return property;
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
  const property = styleText(msg.style);
  p.style.color = currentMessageColor;
  p.style[property] = currentMessageStyle;
  // console.log("p>>>>>", p);
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
    p.style.margin = "0";
    p.padding = "0";
    p.style.color = msgObj.colour;
    const property = styleText(msgObj.style);
    p.style[property] = currentMessageStyle;
    p.textContent = `${msgObj.username}: ${msgObj.message}`;
    historyDiv.appendChild(p);
  });
};

const sendMessage = (event) => {
  const message = document.getElementById("message").value;
  let username = localStorage.getItem("username");
  console.log("username", username);
  if (!username) {
    username = chooseUsername();
  }
  console.log("currentMessageStyle,", currentMessageStyle);
  ws.send(
    JSON.stringify({
      username: username,
      message: message,
      colour: currentMessageColor,
      style: currentMessageStyle,
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

function formatText(command) {
  const selection = window.getSelection();
  if (!selection.rangeCount) return;
  const range = selection.getRangeAt(0);
  const message = document.getElementById("message");
  const property = styleText(command);
  message.style[property] = currentMessageStyle;
  message.style[property].currentMessageStyle;
  range.surroundContents(message);
}

document.querySelectorAll(".format-btn").forEach((button) => {
  button.addEventListener("click", () => {
    const action = button.dataset.action;
    formatText(action);
  });
});

const colorPicker = document.getElementById("text-color-picker");
colorPicker.addEventListener("input", (event) => {
  const selection = window.getSelection();
  if (!selection.rangeCount) return;

  const range = selection.getRangeAt(0);
  const message = document.getElementById("message");
  message.style.color = event.target.value;
  currentMessageColor = event.target.value;
  range.surroundContents(message);
});
