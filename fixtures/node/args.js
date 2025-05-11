console.log("Provided arguments:");
const args = process.argv.slice(2);
if (args.length === 0) {
  console.log("No arguments found");
} else {
  args.forEach((arg, index) => {
    console.log(`${index + 1} => ${arg}`);
  });
}
