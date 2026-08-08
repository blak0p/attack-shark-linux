import { createRoot } from "react-dom/client";
import { App } from "./App";
import "./styles.css";
import { desktopService } from "./wails-service";

createRoot(document.getElementById("root")!).render(<App service={desktopService} />);
