const gracefulShutdown = (signal) => {
  console.log(`Received ${signal}. Shutting down in 5 seconds...`);
  setTimeout(() => {
    console.log("Shutdown timeout reached. Forcing exit now.");
    process.exit(0);
  }, 5000);
};

process.on("SIGINT", () => gracefulShutdown("SIGINT"));
process.on("SIGTERM", () => gracefulShutdown("SIGTERM"));
setInterval(() => {}, 1000);
