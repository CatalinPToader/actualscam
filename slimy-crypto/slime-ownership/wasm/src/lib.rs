// Code generated by the multiversx-sc build system. DO NOT EDIT.

////////////////////////////////////////////////////
////////////////// AUTO-GENERATED //////////////////
////////////////////////////////////////////////////

// Init:                                 1
// Endpoints:                           14
// Async Callback:                       1
// Total number of exported functions:  16

#![no_std]
#![allow(internal_features)]
#![feature(lang_items)]

multiversx_sc_wasm_adapter::allocator!();
multiversx_sc_wasm_adapter::panic_handler!();

multiversx_sc_wasm_adapter::endpoints! {
    slime_ownership
    (
        init => init
        setGeneScienceContractAddress => set_gene_science_contract_address_endpoint
        claim => claim
        totalSupply => total_supply
        balanceOf => balance_of
        ownerOf => owner_of
        approve => approve
        transfer => transfer
        transfer_from => transfer_from
        tokensOfOwner => tokens_of_owner
        createGenZeroSlime => create_gen_zero_slime
        getSlimeById => get_slime_by_id_endpoint
        canBreedWith => can_breed_with
        breedWith => breed_with
        birthFee => birth_fee
    )
}

multiversx_sc_wasm_adapter::async_callback! { slime_ownership }
