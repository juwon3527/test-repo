const form = document.querySelector("#login-form");
const errorBox = document.querySelector("#login-error");
const submitButton = document.querySelector("#login-submit");
const passwordInput = document.querySelector("#password");
const passwordToggle = document.querySelector("#password-toggle");
const signedIn = document.querySelector("#signed-in");

function showError(message) {
  errorBox.textContent = message;
  errorBox.hidden = !message;
}

function showSignedIn(name) {
  document.querySelector("#signed-in-name").textContent = name;
  signedIn.hidden = false;
  form.hidden = true;
}

passwordToggle.addEventListener("click", () => {
  const reveal = passwordInput.type === "password";
  passwordInput.type = reveal ? "text" : "password";
  passwordToggle.textContent = reveal ? "HIDE" : "SHOW";
  passwordToggle.setAttribute("aria-pressed", String(reveal));
});

form.addEventListener("submit", async (event) => {
  event.preventDefault();
  const email = form.email.value.trim();
  const password = form.password.value;
  if (!email || !password) {
    showError("Enter your email and password.");
    return;
  }
  if (!form.email.checkValidity()) {
    showError("Enter a valid email address.");
    return;
  }

  showError("");
  submitButton.disabled = true;
  submitButton.firstChild.textContent = "SIGNING IN… ";
  try {
    const response = await fetch("/api/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email, password }),
    });
    const data = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(data.error || "Sign in failed. Please try again.");
    window.location.assign("/");
  } catch (error) {
    showError(error.message);
    form.password.value = "";
    form.password.focus();
  } finally {
    submitButton.disabled = false;
    submitButton.firstChild.textContent = "SIGN IN ";
  }
});

document.querySelector("#sign-out").addEventListener("click", async () => {
  await fetch("/api/logout", { method: "POST" });
  signedIn.hidden = true;
  form.hidden = false;
  form.email.focus();
});

fetch("/api/session")
  .then((response) => response.json())
  .then((data) => { if (data.authenticated) showSignedIn(data.name); })
  .catch(() => {});
