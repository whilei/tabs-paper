
/*
Run this program with go-ethereum:

    ./build/bin/geth --preload /root/block-intervals.js attach /mnt/myvolume/etc-development.core-geth.classic/geth.ipc

 */
const startBlockNumber = 13000000;
const endBlockNumber = 14000000;

let lastBlock = eth.getBlockByNumber(startBlockNumber - 1, false);

let offsets = {
    0: 0, // 0 seconds, no records (yet!)
    1: 0, // 1 seconds, no records
    2: 0, // 2 seconds, no records
    3: 0,
    // These are just examples. The rest of the potential offsets will be filled on demand.
};

for (let i = startBlockNumber; i <= endBlockNumber; i++) {
    const block = eth.getBlockByNumber(i, false);
    const offset = block.timestamp - lastBlock.timestamp;

    // Increment this offset slot's tally. Offset slot is filled on demand.
    offsets[offset] = offsets[offset] + 1 || 1;

    lastBlock = block;
}

console.log("start block", JSON.stringify(eth.getBlockByNumber(startBlockNumber, false)));
console.log("end block", JSON.stringify(eth.getBlockByNumber(endBlockNumber, false)));
console.log(JSON.stringify(offsets));
