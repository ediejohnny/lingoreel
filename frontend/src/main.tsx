import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App } from "./App";
import { keepDropsInTheApp } from "./drop";
import { backend } from "./services/backend";
import "./styles/app.css";

keepDropsInTheApp(window);
backend.acceptDroppedFiles();

const root = document.getElementById("root");
if (root) {
  createRoot(root).render(
    <StrictMode>
      <App />
    </StrictMode>,
  );
}
