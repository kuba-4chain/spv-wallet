package template

import (
	ec "github.com/bitcoin-sv/go-sdk/primitives/ec"
	script "github.com/bitcoin-sv/go-sdk/script"
	"github.com/bitcoin-sv/go-sdk/script/interpreter"
	"github.com/bitcoin-sv/spv-wallet/engine/spverrors"
	"github.com/bitcoin-sv/spv-wallet/engine/utils"
)

// Evaluate processes a given Bitcoin script by parsing it, replacing certain opcodes
// with the public key hash, and returning the resulting script as a byte array.
// Will replace any OP_PUBKEYHASH or OP_PUBKEY
//
// Parameters:
// - script: A byte array representing the input script.
// - pubKey: A pointer to a bec.PublicKey which provides the dedicated public key to be used in the evaluation.
//
// Returns:
// - A byte array representing the evaluated script, or nil if an error occurs.
func Evaluate(scriptBytes []byte, pubKey *ec.PublicKey) ([]byte, error) {
	s := script.Script(scriptBytes)

	parser := interpreter.DefaultOpcodeParser{}
	parsedScript, err := parser.Parse(&s)
	if err != nil {
		return nil, spverrors.Wrapf(err, "failed to parse script template")
	}

	// Validate parsed opcodes
	for _, op := range parsedScript {
		if op.Value() == 0xFF {
			return nil, spverrors.Newf("invalid opcode")
		}
	}

	// Serialize the public key to compressed format
	dPKBytes := pubKey.Compressed()

	// Apply Hash160 (SHA-256 followed by RIPEMD-160) to the compressed public key
	dPKHash, err := utils.Hash160(dPKBytes)
	if err != nil {
		return nil, spverrors.Wrapf(err, "failed to hash public key")
	}

	// Create a new script with the public key hash
	newScript := new(script.Script)
	if err = newScript.AppendPushData(dPKHash); err != nil {
		return nil, spverrors.Wrapf(err, "failed to convert pubkeyhash value into opcodes")
	}

	// Parse the public key hash script
	pkhParsed, err := parser.Parse(newScript)
	if err != nil {
		return nil, spverrors.Wrapf(err, "failed to convert pubkeyhash value into opcodes")
	}

	// Replace OP_PUBKEYHASH with the actual public key hash
	evaluated := make([]interpreter.ParsedOpcode, 0, len(parsedScript))
	for _, op := range parsedScript {
		switch op.Value() {
		case script.OpPUBKEYHASH:
			evaluated = append(evaluated, pkhParsed...)
		case script.OpPUBKEY:
			return nil, spverrors.Newf("OP_PUBKEY not supported yet")
		default:
			evaluated = append(evaluated, op)
		}
	}

	// Unparse the evaluated opcodes back into a script
	finalScript, err := parser.Unparse(evaluated)
	if err != nil {
		return nil, spverrors.Wrapf(err, "failed to create script after evaluation of template")
	}

	// Cast *bscript.Script back to []byte
	return []byte(*finalScript), nil
}
