# Multi-block message reassembly

An ACARS block carries at most 220 characters of text, so a longer downlink is sent as several blocks. The Airframes feed delivers each block as its own message. `live` parses every block as received and, when the blocks of a message are complete, joins them and parses the joined message as well, about 30 seconds after its last block. The code is in `internal/reassembly`.

## Grouping

A downlink's message number (`message_number` in the feed) is three characters numbering the message and a block letter numbering its blocks: `D47A`, `D47B`, `D47C`. Blocks are grouped by:

- the receiving station (`station.id`): blocks of one message heard by two stations are two receptions, and each is joined on its own;
- the tail;
- the label;
- the first three characters of the message number.

A message without a tail or text, or whose message number does not end in a letter, is not a block of a multi-block downlink and is ignored. Uplinks carry no message number, so they are not reassembled.

## When a message is complete

The feed's `block_end` flag does not mark the last block reliably. In a capture of 8,000 feed messages in October 2026, it was false on most single-block messages, and true on every block of some multi-block messages and false on every block of others. Completeness is therefore inferred from the blocks themselves, and only once no block of the message has arrived for 30 seconds (the settling time), so that a block arriving later can still change the judgement. `live` judges the settled messages every 5 seconds. A message is then:

- **complete**, and joined, when there is a block for every letter from A to the highest one received (at least two), every block but the last has the same length as block A (a full block: 206 or 210 characters for H1 in the capture, 176 for some messages), and the last block is shorter;
- **inconsistent**, and never joined, when a block other than the last is not full, a block is longer than block A, or the same letter was received twice with different text (two receptions that cannot both be right, so neither is trusted). A message of a single short block A needs no joining and is treated the same way;
- **waiting**, when the blocks so far are consistent but the message may have more: the last block is full, or a letter is missing. It is judged again at each flush until a block completes it or it is forgotten.

A message whose last block happens to be full is therefore never joined, and neither is one with a block that never arrives: a truncated message would be worse than none. A block received twice with the same text is one block.

## Joining

The blocks' texts are joined in letter order. Some feeders repeat an H1 message's FMC preamble (`#DFB`, `- #CFB`; see [parsers.md](parsers.md#dispatch-order)) at the start of every block; a later block's copy of the first block's preamble is dropped. Text of other labels is joined unchanged.

The joined message takes its fields from block A. Its flight is the first block's that has one.

| Field | Value |
|---|---|
| `id` | Block A's ID with bit 62 set (`reassembly.AssembledIDFlag`). Airframes' IDs were about 8×10⁹ in October 2026, far below 2⁶², so a joined message's ID is never a received one. |
| `message_number` | The three characters, without a block letter (`D47`) |
| `assembled_from` | The blocks' IDs, in order |
| `block_end` | Not set |

It is parsed, updates the state and is stored like any received message.

## Messages a feeder has already joined

Some feeders also send the whole message, joined, as one "block" longer than an ACARS block can be (in the capture, 5 of 81 multi-block messages, up to 2,094 characters, sent with the last block). That copy is parsed as received. When a block longer than 220 characters arrives before the message has settled, the message's blocks are not joined. A copy that arrives after the joined message has been processed is still processed as received, so in that case the message is parsed twice.

## Memory

Every downlink starts a group, because a single-block message's number also ends in `A`. A group with no new block for two minutes is forgotten (blocks of one message arrived up to about 25 seconds apart in the capture); a block that arrives after that starts a new group rather than joining the old blocks. At most 50,000 groups are kept, the least recently updated being forgotten first. Groups are kept in memory only, so blocks pending when `live` stops are lost.

## The archive

Only messages stored since the block columns were added (October 2026) have a message number. Earlier rows have none, so their blocks cannot be grouped and are not joined.
